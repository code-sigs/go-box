package minio

import (
	"context"
	"fmt"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

type MinIOConfig struct {
	Endpoint  string `mapstructure:"endpoint"`
	AccessKey string `mapstructure:"accessKey"`
	SecretKey string `mapstructure:"secretKey"`
	UseSSL    bool   `mapstructure:"useSSL"`
	Bucket    string `mapstructure:"bucket"`
	IsPublic  bool   `mapstructure:"isPublic"`
}

type MinIO struct {
	client   *minio.Client
	bucket   string
	endpoint string
	useSSL   bool
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

	return &MinIO{
		client:   client,
		bucket:   cfg.Bucket,
		endpoint: cfg.Endpoint,
		useSSL:   cfg.UseSSL,
	}, nil
}

func (m *MinIO) UploadFile(objectName string, reader io.Reader, size int64, contentType string) (string, error) {
	_, err := m.client.PutObject(context.Background(), m.bucket, objectName, reader, size, minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		return "", fmt.Errorf("failed to upload file: %w", err)
	}

	scheme := "http"
	if m.useSSL {
		scheme = "https"
	}

	url := fmt.Sprintf("%s://%s/%s/%s", scheme, m.endpoint, m.bucket, objectName)
	return url, nil
}

// UploadLocalFile 从本地路径上传文件并自动识别 contentType
func (m *MinIO) UploadLocalFile(objectName, filePath string) (string, error) {
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
	return m.UploadFile(objectName, file, stat.Size(), contentType)
}
