package s3

import (
	"errors"
	"strings"

	"github.com/Alyanaky/SecureDAG/internal/storage"
)

type S3Adapter struct {
	store         *storage.PostgresStore
	storageBackend *storage.BadgerStore
}

func NewS3Adapter(store *storage.PostgresStore, backend *storage.BadgerStore) *S3Adapter {
	return &S3Adapter{
		store:         store,
		storageBackend: backend,
	}
}

func (a *S3Adapter) PutObject(bucket, key string, data []byte, metadata map[string]string) error {
	hash, err := a.storageBackend.PutBlockWithMetadata(data, metadata)
	if err != nil {
		return err
	}
	obj := &storage.Object{
		Bucket:   bucket,
		Key:      key,
		Hash:     hash,
		Metadata: metadata,
	}
	return a.store.PutObject(obj)
}

func (a *S3Adapter) GetObject(bucket, key, versionID string) (*storage.Object, error) {
	return a.store.GetObject(bucket, key, versionID)
}

func (a *S3Adapter) DeleteObject(bucket, key, versionID string) error {
	return a.store.DeleteObject(bucket, key, versionID)
}

func (a *S3Adapter) CreateMultipartUpload(bucket, key string) (string, error) {
	return a.store.CreateMultipartUpload(bucket, key)
}

func (a *S3Adapter) UploadPart(bucket, key, uploadID string, partNumber int, data []byte) (string, error) {
	hash, err := a.storageBackend.PutBlock(data)
	if err != nil {
		return "", err
	}
	return a.store.UploadPart(bucket, key, uploadID, partNumber, hash)
}

func (a *S3Adapter) CompleteMultipartUpload(bucket, key, uploadID string, parts []storage.Part) error {
	return a.store.CompleteMultipartUpload(bucket, key, uploadID, parts)
}

func (a *S3Adapter) AbortMultipartUpload(bucket, key, uploadID string) error {
	return a.store.AbortMultipartUpload(uploadID)
}

func (a *S3Adapter) CopyObject(destBucket, destKey, source string) error {
	parts := strings.Split(source, "/")
	if len(parts) < 2 || parts[0] != "" {
		return errors.New("invalid copy source")
	}
	sourceBucket := parts[1]
	sourceKey := strings.Join(parts[2:], "/")
	sourceObj, err := a.store.GetObject(sourceBucket, sourceKey, "")
	if err != nil {
		return err
	}
	data, err := a.storageBackend.GetBlock(sourceObj.Hash)
	if err != nil {
		return err
	}
	hash, err := a.storageBackend.PutBlockWithMetadata(data, sourceObj.Metadata)
	if err != nil {
		return err
	}
	destObj := &storage.Object{
		Bucket:   destBucket,
		Key:      destKey,
		Hash:     hash,
		Metadata: sourceObj.Metadata,
	}
	return a.store.PutObject(destObj)
}
