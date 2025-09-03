package minio

import (
	"context"
	"fmt"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"time"
)

type MinIOConfig struct {
	Endpoint     string `mapstructure:"endpoint"`
	AccessKey    string `mapstructure:"accessKey"`
	SecretKey    string `mapstructure:"secretKey"`
	UseSSL       bool   `mapstructure:"useSSL"`
	Bucket       string `mapstructure:"bucket"`
	IsPublic     bool   `mapstructure:"isPublic"`
	ExternalAddr string `mapstructure:"externalAddr"`
}

type MinIO struct {
	client *minio.Client
	cfg    *MinIOConfig
}

func NewMinIO(cfg *MinIOConfig) (*MinIO, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create MinIO client: %w", err)
	}

	// 可选：检查 Bucket 是否存在
	exists, err := client.BucketExists(context.Background(), cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("failed to check bucket existence: %w", err)
	}

	if !exists {
		err = client.MakeBucket(context.Background(), cfg.Bucket, minio.MakeBucketOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to create bucket: %w", err)
		}
		// 设置 bucket 权限
		if cfg.IsPublic {
			policy := fmt.Sprintf(`{
											"Version":"2012-10-17",
											"Statement":[
												{
													"Effect":"Allow",
													"Principal":{"AWS":["*"]},
													"Action":["s3:GetObject"],
													"Resource":["arn:aws:s3:::%s/*"]
												}
											]
										}`, cfg.Bucket)

			err = client.SetBucketPolicy(context.Background(), cfg.Bucket, policy)
			if err != nil {
				return nil, fmt.Errorf("failed to set public read-only bucket policy: %w", err)
			}
		}

	}
	if cfg.ExternalAddr == "" {
		cfg.ExternalAddr = cfg.Endpoint
	}
	return &MinIO{
		client: client,
		cfg:    cfg,
	}, nil
}

func (m *MinIO) UploadFile(ctx context.Context, objectName string, reader io.Reader, size int64, contentType string) (string, error) {
	_, err := m.client.PutObject(ctx, m.cfg.Bucket, objectName, reader, size, minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		return "", fmt.Errorf("failed to upload file: %w", err)
	}

	//scheme := "http"
	//if m.cfg.UseSSL {
	//	scheme = "https"
	//}

	//url := fmt.Sprintf("%s/%s/%s", m.cfg.Endpoint, m.cfg.Bucket, objectName)
	return fmt.Sprintf("%s/%s/%s", m.cfg.Endpoint, m.cfg.Bucket, objectName), nil
}

// UploadLocalFile 从本地路径上传文件并自动识别 contentType
func (m *MinIO) UploadLocalFile(ctx context.Context, objectName, filePath string) (string, error) {
	// 打开本地文件
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open local file: %w", err)
	}
	defer file.Close()

	// 获取文件信息
	stat, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("failed to stat local file: %w", err)
	}

	// 读取前 512 字节用于 MIME 类型识别
	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("failed to read file header for content type: %w", err)
	}

	// 自动检测 content-type
	contentType := http.DetectContentType(buffer[:n])

	// 重置文件指针位置
	_, err = file.Seek(0, io.SeekStart)
	if err != nil {
		return "", fmt.Errorf("failed to seek file: %w", err)
	}

	// 如果 objectName 为空，使用文件名
	if objectName == "" {
		objectName = filepath.Base(filePath)
	}

	// 调用上传方法
	return m.UploadFile(ctx, objectName, file, stat.Size(), contentType)
}

func (m *MinIO) PresignedPutURL(ctx context.Context, objectName string, expiry time.Duration) (string, string, error) {
	if expiry <= 0 {
		expiry = time.Hour
	}

	presignedURL, err := m.client.PresignedPutObject(ctx, m.cfg.Bucket, objectName, expiry)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate presigned upload URL: %w", err)
	}
	// 使用 ExternalHost 替换原有 Host
	u, err := url.Parse(presignedURL.String())
	if err != nil {
		return "", "", fmt.Errorf("failed to parse presigned URL: %w", err)
	}

	externalURL, err := url.Parse(m.cfg.ExternalAddr)
	if err != nil {
		return "", "", fmt.Errorf("invalid ExternalAddr: %w", err)
	}

	// 替换 scheme 和 host
	u.Scheme = externalURL.Scheme
	u.Host = externalURL.Host
	return u.String(), path.Join(m.cfg.Bucket, objectName), nil
}

func (m *MinIO) PresignedGetURL(ctx context.Context, objectName string, expiry time.Duration, filename string, inline bool, contentType string) (string, error) {
	if expiry <= 0 {
		expiry = time.Hour
	}
	reqParams := make(url.Values)
	if filename != "" {
		disposition := "attachment"
		if inline {
			disposition = "inline"
		}
		safeFileName := url.PathEscape(filename)
		reqParams.Set("response-content-disposition", fmt.Sprintf("%s; filename=\"%s\"", disposition, safeFileName))
	}
	if contentType != "" {
		reqParams.Set("response-content-type", contentType)
	}
	presignedURL, err := m.client.PresignedGetObject(ctx, m.cfg.Bucket, objectName, expiry, reqParams)
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned download URL: %w", err)
	}
	// 使用 ExternalHost 替换原有 Host
	u, err := url.Parse(presignedURL.String())
	if err != nil {
		return "", fmt.Errorf("failed to parse presigned URL: %w", err)
	}

	externalURL, err := url.Parse(m.cfg.ExternalAddr)
	if err != nil {
		return "", fmt.Errorf("invalid ExternalHost: %w", err)
	}

	// 替换 scheme 和 host
	u.Scheme = externalURL.Scheme
	u.Host = externalURL.Host

	return u.String(), nil
}

func (m *MinIO) MoveObject(ctx context.Context, srcObject, dstObject string) (string, error) {
	src := minio.CopySrcOptions{
		Bucket: m.cfg.Bucket,
		Object: srcObject,
	}
	dst := minio.CopyDestOptions{
		Bucket: m.cfg.Bucket,
		Object: dstObject,
	}

	_, err := m.client.CopyObject(ctx, dst, src)
	if err != nil {
		return "", fmt.Errorf("failed to copy object: %w", err)
	}

	err = m.client.RemoveObject(ctx, m.cfg.Bucket, srcObject, minio.RemoveObjectOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to delete source object: %w", err)
	}

	return path.Join(m.cfg.Bucket, dstObject), nil
}
