package minio

import (
	"context"
	"io"
	"time"
)

type ObjectStorage interface {
	UploadFile(ctx context.Context, bucketName, objectName string, reader io.Reader, size int64, contentType string) (string, error)
	PresignedPutURL(ctx context.Context, objectName string, expiry time.Duration) (string, string, error)
	PresignedGetURL(ctx context.Context, objectName string, expiry time.Duration, filename string, inline bool, contentType string) (string, error)
}
