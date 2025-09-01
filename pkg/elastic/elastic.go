package elastic

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/elastic/go-elasticsearch/v9"
	"github.com/elastic/go-elasticsearch/v9/esapi"
	"io"
	"net/http"
	"strings"
	"time"
)

// ElasticConfig 定义 Elasticsearch 客户端的配置参数
type ElasticConfig struct {
	Hosts          []string     `mapstructure:"hosts"`          // ES 节点地址
	Username       string       `mapstructure:"username"`       // 用户名
	Password       string       `mapstructure:"password"`       // 密码
	Healthcheck    bool         `mapstructure:"healthcheck"`    // 是否启用健康检查
	RetryOnFailure int          `mapstructure:"retryOnFailure"` // 失败重试次数
	Timeout        int64        `mapstructure:"timeout"`        // 超时时间（毫秒）
	HTTPClient     *http.Client // 可选 HTTP 客户端（用于 TLS/超时/测试）
}

// IndexNamer 接口要求实现获取基础索引名的方法
type IndexNamer interface {
	IndexName() string
}

// IndexStrategy 定义索引命名策略，根据基础索引名生成最终索引名
type IndexStrategy func(base string) string

// 常见索引命名策略
func DefaultIndexStrategy(base string) string { return base }
func YearlyIndexStrategy(base string) string  { return fmt.Sprintf("%s-%d", base, time.Now().Year()) }
func MonthlyIndexStrategy(base string) string {
	return fmt.Sprintf("%s-%s", base, time.Now().Format("2006.01"))
}

// ElasticClient 是用于处理实现 IndexNamer 接口的文档的 Elasticsearch 客户端
type ElasticClient[T IndexNamer] struct {
	es     *elasticsearch.Client
	config *ElasticConfig
}

// NewElasticClient 创建并初始化 ES 客户端（不会 panic）
func NewElasticClient[T IndexNamer](cfg *ElasticConfig) (*ElasticClient[T], error) {
	esCfg := elasticsearch.Config{
		Addresses: cfg.Hosts,
		Username:  cfg.Username,
		Password:  cfg.Password,
	}
	if cfg.HTTPClient != nil {
		esCfg.Transport = cfg.HTTPClient.Transport
	}

	client, err := elasticsearch.NewClient(esCfg)
	if err != nil {
		return nil, fmt.Errorf("创建 elastic client 失败: %w", err)
	}

	// 通过 Info 接口进行轻量级健康检查
	res, err := client.Info()
	if err != nil {
		return nil, fmt.Errorf("连接 elastic 失败: %w", err)
	}
	defer res.Body.Close()
	if res.IsError() {
		b, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("elastic info 错误: %s", string(b))
	}

	return &ElasticClient[T]{es: client, config: cfg}, nil
}

// 内部辅助函数：执行请求带超时和重试
func (c *ElasticClient[T]) doRequestWithRetry(ctx context.Context, fn func(ctx context.Context) (*esapi.Response, error)) (*esapi.Response, error) {
	timeout := c.config.Timeout
	if timeout <= 0 {
		timeout = 10000 // 默认 10 秒
	}
	retries := c.config.RetryOnFailure
	if retries <= 0 {
		retries = 3
	}

	var lastErr error
	for i := 0; i < retries; i++ {
		ctxTimeout, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Millisecond)
		res, err := fn(ctxTimeout)
		cancel()
		if err == nil && res != nil && !res.IsError() {
			return res, nil
		}
		if err != nil {
			lastErr = err
		}
		if res != nil && res.IsError() {
			b, _ := io.ReadAll(res.Body)
			lastErr = fmt.Errorf("ES请求错误: %s", string(b))
		}
		time.Sleep(time.Duration(200*(i+1)) * time.Millisecond)
	}
	return nil, fmt.Errorf("请求失败重试 %d 次仍失败: %w", retries, lastErr)
}

// CreateDocument 索引单个文档。id 可为空（由 ES 自动生成）
func (c *ElasticClient[T]) CreateDocument(ctx context.Context, doc *T, id string, strategy IndexStrategy) error {
	if doc == nil {
		return errors.New("文档为空")
	}
	if strategy == nil {
		strategy = DefaultIndexStrategy
	}
	index := strategy((*doc).IndexName())

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(doc); err != nil {
		return fmt.Errorf("编码文档失败: %w", err)
	}

	req := esapi.IndexRequest{
		Index:      index,
		DocumentID: id,
		Body:       &buf,
		Refresh:    "true",
	}

	res, err := c.doRequestWithRetry(ctx, func(ctx context.Context) (*esapi.Response, error) {
		return req.Do(ctx, c.es)
	})
	if err != nil {
		return err
	}
	defer res.Body.Close()
	return nil
}

// BulkCreateDocuments 批量索引文档
func (c *ElasticClient[T]) BulkCreateDocuments(ctx context.Context, docs []*T, idForDoc func(*T) string, strategy IndexStrategy) error {
	if len(docs) == 0 {
		return nil
	}
	if strategy == nil {
		strategy = DefaultIndexStrategy
	}
	index := strategy((*(docs[0])).IndexName())

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	for _, d := range docs {
		meta := map[string]map[string]interface{}{"index": {"_index": index}}
		if idForDoc != nil {
			if id := idForDoc(d); id != "" {
				meta["index"]["_id"] = id
			}
		}
		if err := enc.Encode(meta); err != nil {
			return fmt.Errorf("编码批量索引 meta 失败: %w", err)
		}
		if err := enc.Encode(d); err != nil {
			return fmt.Errorf("编码批量文档失败: %w", err)
		}
	}

	res, err := c.doRequestWithRetry(ctx, func(ctx context.Context) (*esapi.Response, error) {
		return c.es.Bulk(&buf, c.es.Bulk.WithContext(ctx), c.es.Bulk.WithRefresh("true"))
	})
	if err != nil {
		return err
	}
	defer res.Body.Close()

	var r map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&r); err == nil {
		if errorsField, ok := r["errors"].(bool); ok && errorsField {
			return fmt.Errorf("批量操作包含错误: %v", r)
		}
	}
	return nil
}

// BulkDeleteDocuments 批量删除文档
func (c *ElasticClient[T]) BulkDeleteDocuments(ctx context.Context, baseIndex string, ids []string, strategy IndexStrategy) error {
	if len(ids) == 0 {
		return nil
	}
	if baseIndex == "" {
		var zero T
		baseIndex = zero.IndexName() + "-*"
	}
	if strategy == nil {
		strategy = DefaultIndexStrategy
	}
	index := strategy(baseIndex)

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	for _, id := range ids {
		meta := map[string]map[string]interface{}{"delete": {"_index": index, "_id": id}}
		if err := enc.Encode(meta); err != nil {
			return fmt.Errorf("编码批量删除 meta 失败: %w", err)
		}
	}

	req := esapi.BulkRequest{
		Index:   index,
		Body:    &buf,
		Refresh: "true",
	}
	res, err := c.doRequestWithRetry(ctx, func(ctx context.Context) (*esapi.Response, error) {
		return req.Do(ctx, c.es)
	})
	if err != nil {
		return err
	}
	defer res.Body.Close()

	var r map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&r); err == nil {
		if items, ok := r["items"].([]interface{}); ok {
			for _, it := range items {
				m, ok := it.(map[string]interface{})
				if !ok {
					continue
				}
				if del, ok := m["delete"].(map[string]interface{}); ok {
					if status, ok := del["status"].(float64); ok && status >= 300 {
						return fmt.Errorf("删除失败: %v", del)
					}
				}
			}
		}
	}
	return nil
}

// DeleteDocument 删除单个文档
func (c *ElasticClient[T]) DeleteDocument(ctx context.Context, baseIndex, id string) error {
	if id == "" {
		return errors.New("ID 不能为空")
	}
	if baseIndex == "" {
		var zero T
		baseIndex = zero.IndexName() + "-*"
	}
	req := esapi.DeleteRequest{
		Index:      baseIndex,
		DocumentID: id,
		Refresh:    "true",
	}
	res, err := c.doRequestWithRetry(ctx, func(ctx context.Context) (*esapi.Response, error) {
		return req.Do(ctx, c.es)
	})
	if err != nil {
		return err
	}
	defer res.Body.Close()
	return nil
}

// Search 执行搜索请求
func (c *ElasticClient[T]) Search(ctx context.Context, query map[string]interface{}, indices ...string) ([]*T, int64, error) {
	if len(indices) == 0 {
		var zero T
		indices = []string{zero.IndexName() + "-*"}
	}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(query); err != nil {
		return nil, 0, fmt.Errorf("编码查询参数失败: %w", err)
	}

	res, err := c.doRequestWithRetry(ctx, func(ctx context.Context) (*esapi.Response, error) {
		return c.es.Search(c.es.Search.WithContext(ctx), c.es.Search.WithIndex(indices...), c.es.Search.WithBody(&buf))
	})
	if err != nil {
		return nil, 0, err
	}
	defer res.Body.Close()

	var result struct {
		Hits struct {
			Total struct {
				Value int64 `json:"value"`
			} `json:"total"`
			Hits []struct {
				Source *T `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, 0, fmt.Errorf("解析搜索结果失败: %w", err)
	}

	out := make([]*T, 0, len(result.Hits.Hits))
	for _, h := range result.Hits.Hits {
		out = append(out, h.Source)
	}
	return out, result.Hits.Total.Value, nil
}

// SearchPagination 支持 search_after 分页
// sortFields 的格式是 []string{"@timestamp:desc", "id:asc"}
func (c *ElasticClient[T]) PaginateSearch(
	ctx context.Context,
	query map[string]interface{},
	sortFields []string,
	size int,
	cursor string,
	startTime, endTime *time.Time,
) ([]*T, string, int64, error) {

	// 1. 确定索引模式
	var zero T
	baseIndex := zero.IndexName() + "-*"

	// 2. 构建查询 DSL
	if query == nil {
		query = make(map[string]interface{})
	}
	boolQuery := map[string]interface{}{
		"must": []interface{}{},
	}
	if q, ok := query["query"]; ok {
		boolQuery["must"] = append(boolQuery["must"].([]interface{}), q)
	}

	// 时间过滤
	if startTime != nil || endTime != nil {
		rangeQuery := map[string]interface{}{
			"range": map[string]interface{}{
				"@timestamp": map[string]interface{}{},
			},
		}
		if startTime != nil {
			rangeQuery["range"].(map[string]interface{})["@timestamp"].(map[string]interface{})["gte"] = startTime.Format(time.RFC3339)
		}
		if endTime != nil {
			rangeQuery["range"].(map[string]interface{})["@timestamp"].(map[string]interface{})["lte"] = endTime.Format(time.RFC3339)
		}
		boolQuery["must"] = append(boolQuery["must"].([]interface{}), rangeQuery)
	}

	// 组装最终查询
	dsl := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": boolQuery,
		},
		"size": size,
	}

	// 排序字段
	if len(sortFields) > 0 {
		var sorts []map[string]interface{}
		for _, sf := range sortFields {
			field := sf
			order := "asc"
			if strings.Contains(sf, ":") {
				parts := strings.Split(sf, ":")
				field = parts[0]
				order = parts[1]
			}
			sorts = append(sorts, map[string]interface{}{field: map[string]string{"order": order}})
		}
		dsl["sort"] = sorts
	}

	// 游标（search_after）
	if cursor != "" {
		decoded, err := base64.URLEncoding.DecodeString(cursor)
		if err != nil {
			return nil, "", 0, fmt.Errorf("解码游标失败: %w", err)
		}
		var sa []interface{}
		if err := json.Unmarshal(decoded, &sa); err != nil {
			return nil, "", 0, fmt.Errorf("解析游标失败: %w", err)
		}
		dsl["search_after"] = sa
	}

	// 3. 发送请求
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(dsl); err != nil {
		return nil, "", 0, fmt.Errorf("编码查询失败: %w", err)
	}

	res, err := c.doRequestWithRetry(ctx, func(ctx context.Context) (*esapi.Response, error) {
		return c.es.Search(
			c.es.Search.WithContext(ctx),
			c.es.Search.WithIndex(baseIndex),
			c.es.Search.WithBody(&buf),
		)
	})
	if err != nil {
		return nil, "", 0, err
	}
	defer res.Body.Close()

	// 4. 解析响应
	var raw struct {
		Hits struct {
			Total struct {
				Value int64 `json:"value"`
			} `json:"total"`
			Hits []struct {
				Source json.RawMessage `json:"_source"`
				Sort   []interface{}   `json:"sort"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(res.Body).Decode(&raw); err != nil {
		return nil, "", 0, fmt.Errorf("解析结果失败: %w", err)
	}

	// 5. 反序列化文档
	docs := make([]*T, 0, len(raw.Hits.Hits))
	for _, h := range raw.Hits.Hits {
		var doc T
		if err := json.Unmarshal(h.Source, &doc); err == nil {
			docs = append(docs, &doc)
		}
	}

	// 6. 生成新游标
	nextCursor := ""
	if len(raw.Hits.Hits) == size {
		lastSort := raw.Hits.Hits[len(raw.Hits.Hits)-1].Sort
		sortBytes, _ := json.Marshal(lastSort)
		nextCursor = base64.URLEncoding.EncodeToString(sortBytes)
	}

	return docs, nextCursor, raw.Hits.Total.Value, nil
}
