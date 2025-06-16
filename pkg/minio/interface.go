package minio

import "io"

type ObjectStorage interface {
	UploadFile(bucketName, objectName string, reader io.Reader, size int64, contentType string) (string, error)
}
