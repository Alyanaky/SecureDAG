package s3

import (
    "bytes"
    "context"
    "io"

    "github.com/Alyanaky/SecureDAG/internal/storage"
    "github.com/aws/aws-sdk-go-v2/service/s3"
    "github.com/aws/aws-sdk-go-v2/aws"
)

type S3Adapter struct {
    storageBackend *storage.BadgerStore
    store         *storage.PostgresStore
}

func NewS3Adapter(storageBackend *storage.BadgerStore, store *storage.PostgresStore) *S3Adapter {
    return &S3Adapter{
        storageBackend: storageBackend,
        store:         store,
    }
}

func (a *S3Adapter) PutObject(ctx context.Context, input *s3.PutObjectInput) (*s3.PutObjectOutput, error) {
    data, err := io.ReadAll(input.Body)
    if err != nil {
        return nil, err
    }
    err = a.storageBackend.PutObject(*input.Bucket, *input.Key, data)
    if err != nil {
        return nil, err
    }
    return &s3.PutObjectOutput{}, nil
}

func (a *S3Adapter) GetObject(ctx context.Context, input *s3.GetObjectInput) (*s3.GetObjectOutput, error) {
    data, err := a.storageBackend.GetObject(*input.Bucket, *input.Key)
    if err != nil {
        return nil, err
    }
    return &s3.GetObjectOutput{
        Body: io.NopCloser(bytes.NewReader(data)),
    }, nil
}

func (a *S3Adapter) DeleteObject(ctx context.Context, input *s3.DeleteObjectInput) (*s3.DeleteObjectOutput, error) {
    err := a.storageBackend.DeleteObject(*input.Bucket, *input.Key)
    if err != nil {
        return nil, err
    }
    return &s3.DeleteObjectOutput{}, nil
}

func (a *S3Adapter) CreateMultipartUpload(ctx context.Context, input *s3.CreateMultipartUploadInput) (*s3.CreateMultipartUploadOutput, error) {
    return &s3.CreateMultipartUploadOutput{
        UploadId: aws.String("mock-upload-id"),
    }, nil
}

func (a *S3Adapter) UploadPart(ctx context.Context, input *s3.UploadPartInput) (*s3.UploadPartOutput, error) {
    return &s3.UploadPartOutput{
        ETag: aws.String("mock-etag"),
    }, nil
}
