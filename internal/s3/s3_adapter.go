package s3

import (
	"bytes"
	"crypto/md5"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Alyanaky/SecureDAG/internal/storage"
)

const (
	MaxMultipartParts = 10000
	MaxPartSize       = 5 << 30 // 5GB
)

type S3Adapter struct {
	store storage.Store
}

func NewS3Adapter(store storage.Store) *S3Adapter {
	return &S3Adapter{store: store}
}

// Основные операции с объектами

func (a *S3Adapter) PutObject(bucket, key string, data []byte, metadata map[string]string) (string, error) {
	if !a.store.BucketExists(bucket) {
		return "", NewS3Error(NoSuchBucket, bucket)
	}

	meta := map[string]string{
		"Bucket":       bucket,
		"Key":          key,
		"Content-Type": metadata["Content-Type"],
	}
	
	for k, v := range metadata {
		if strings.HasPrefix(k, "x-amz-meta-") {
			meta[k[11:]] = v
		}
	}

	hash, err := a.store.PutBlockWithMetadata(data, meta)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("\"%x\"", md5.Sum(data)), nil
}

func (a *S3Adapter) GetObject(bucket, key, versionID string) ([]byte, map[string]string, error) {
	if !a.store.BucketExists(bucket) {
		return nil, nil, NewS3Error(NoSuchBucket, bucket)
	}

	data, meta, err := a.store.GetBlockWithMetadata(versionID)
	if err != nil || meta["Key"] != key {
		return nil, nil, NewS3Error(NoSuchKey, key)
	}

	return data, processMetadata(meta), nil
}

// Multipart Upload

type MultipartUpload struct {
	UploadID  string
	Bucket    string
	Key       string
	Initiated time.Time
}

func (a *S3Adapter) CreateMultipartUpload(bucket, key string) (string, error) {
	uploadID := generateUploadID(bucket, key)
	err := a.store.CreateMultipartSession(uploadID, bucket, key)
	return uploadID, err
}

func (a *S3Adapter) UploadPart(bucket, key, uploadID string, partNumber int, data []byte) (string, error) {
	session, err := a.store.GetMultipartSession(uploadID)
	if err != nil {
		return "", NewS3Error(NoSuchUpload, uploadID)
	}

	if partNumber < 1 || partNumber > MaxMultipartParts {
		return "", NewS3Error(InvalidPart, "")
	}

	etag := fmt.Sprintf("%x", md5.Sum(data))
	err = a.store.SavePart(uploadID, partNumber, etag, data)
	return etag, err
}

func (a *S3Adapter) CompleteMultipartUpload(uploadID string, parts []Part) (string, error) {
	session, err := a.store.GetMultipartSession(uploadID)
	if err != nil {
		return "", NewS3Error(NoSuchUpload, uploadID)
	}

	var buffer bytes.Buffer
	for _, part := range parts {
		data, err := a.store.GetPart(uploadID, part.PartNumber)
		if err != nil {
			return "", err
		}
		buffer.Write(data)
	}

	finalETag, err := a.PutObject(session.Bucket, session.Key, buffer.Bytes(), nil)
	if err != nil {
		return "", err
	}

	a.store.DeleteMultipartSession(uploadID)
	return finalETag, nil
}

// Вспомогательные методы

func generateUploadID(bucket, key string) string {
	h := md5.New()
	h.Write([]byte(fmt.Sprintf("%s-%s-%d", bucket, key, time.Now().UnixNano())))
	return fmt.Sprintf("%x", h.Sum(nil))
}

func processMetadata(meta map[string]string) map[string]string {
	result := make(map[string]string)
	for k, v := range meta {
		if k == "Bucket" || k == "Key" {
			continue
		}
		result["x-amz-meta-"+k] = v
	}
	return result
}

// Обработка ошибок S3

type S3Error struct {
	Code      string
	Message   string
	Resource  string
	RequestID string
}

func (e S3Error) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func NewS3Error(code ErrorCode, resource string) *S3Error {
	return &S3Error{
		Code:      string(code),
		Message:   code.Description(),
		Resource:  resource,
		RequestID: generateRequestID(),
	}
}

func generateRequestID() string {
	return fmt.Sprintf("%x", md5.Sum([]byte(time.Now().String())))
}

type ErrorCode string

const (
	NoSuchBucket  ErrorCode = "NoSuchBucket"
	NoSuchKey     ErrorCode = "NoSuchKey"
	NoSuchUpload  ErrorCode = "NoSuchUpload"
	InvalidPart   ErrorCode = "InvalidPart"
	InternalError ErrorCode = "InternalError"
)

func (c ErrorCode) Description() string {
	switch c {
	case NoSuchBucket:
		return "The specified bucket does not exist"
	case NoSuchKey:
		return "The specified key does not exist"
	case NoSuchUpload:
		return "The specified multipart upload does not exist"
	case InvalidPart:
		return "The part number is invalid"
	default:
		return "Internal server error"
	}
}
