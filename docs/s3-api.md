# SecureDAG S3-Compatible API Documentation

## Base URL
`http://<server-address>/s3`

## Authentication
```http
Authorization: Bearer <JWT_TOKEN>
```

## Bucket Operations

### Create Bucket
```http
PUT /{bucket}
```

### Delete Bucket
```http
DELETE /{bucket}
```

### List Buckets
```http
GET /
```

## Object Operations

### Put Object
```http
PUT /{bucket}/{key}
```
**Headers:**
- `Content-Type`: MIME type
- `x-amz-meta-*`: Custom metadata

**Success Response:**
```xml
<PutObjectResult>
  <ETag>"d41d8cd98f00b204e9800998ecf8427e"</ETag>
</PutObjectResult>
```

### Get Object
```http
GET /{bucket}/{key}?versionId=<VERSION_ID>
```

### Delete Object
```http
DELETE /{bucket}/{key}?versionId=<VERSION_ID>
```

## Versioning

### Enable Versioning
```http
PUT /{bucket}?versioning
```

### List Object Versions
```http
GET /{bucket}/{key}?versions
```

**Example Response:**
```xml
<ListVersionsResult>
  <Version>
    <Key>report.pdf</Key>
    <VersionId>ABC123</VersionId>
    <IsLatest>true</IsLatest>
    <LastModified>2023-10-05T12:00:00Z</LastModified>
    <ETag>"d41d8cd98f00b204e9800998ecf8427e"</ETag>
    <Size>2048</Size>
  </Version>
</ListVersionsResult>
```

## Multipart Upload

### Initiate Upload
```http
POST /{bucket}/{key}?uploads
```

### Upload Part
```http
PUT /{bucket}/{key}?partNumber=1&uploadId=UPLOAD_ID
```

### Complete Upload
```http
POST /{bucket}/{key}?uploadId=UPLOAD_ID
```

## Lifecycle Management

### Add Lifecycle Rule
```http
PUT /admin/lifecycle
{
  "id": "delete-old-files",
  "actions": ["Delete"],
  "conditions": {"ageDays": 30}
}
```

### Execute Rules
```http
POST /admin/lifecycle/execute?bucket=<BUCKET>
```

## Error Responses
```xml
<Error>
  <Code>NoSuchKey</Code>
  <Message>The specified key does not exist</Message>
  <Resource>/my-bucket/missing-file.txt</Resource>
  <RequestId>REQ123</RequestId>
</Error>
```

| Code            | Description                     |
|-----------------|---------------------------------|
| AccessDenied    | Permission denied               |
| NoSuchBucket    | Bucket does not exist           |
| NoSuchKey       | Object not found                |
| InvalidArgument | Invalid request parameters      |
