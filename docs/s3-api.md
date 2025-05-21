# SecureDAG S3-Compatible API Documentation

## Base URL
`http://<server-address>/s3`

## Authentication
Include JWT token in Authorization header:
```http
Authorization: Bearer <your-token>
```

## Bucket Operations

### Create Bucket
```http
PUT /{bucket}
```

**Response**
- 200 OK: Bucket created
- 409 Conflict: Bucket already exists

## Object Operations

### Put Object
```http
PUT /{bucket}/{key}
```

**Headers**
- `Content-Type`: MIME type of object
- `x-amz-meta-*`: Custom metadata

**Example**
```bash
curl -X PUT -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: text/plain" \
  -H "x-amz-meta-author: john" \
  -T file.txt \
  http://localhost:8080/s3/my-bucket/docs/file.txt
```

**Response**
- 200 OK: 
  ```xml
  <PutObjectResult>
    <ETag>"d41d8cd98f00b204e9800998ecf8427e"</ETag>
  </PutObjectResult>
  ```

### Get Object
```http
GET /{bucket}/{key}
```

**Query Parameters**
- `versionId`: Object version ID (required)

**Example**
```bash
curl -H "Authorization: Bearer TOKEN" \
  "http://localhost:8080/s3/my-bucket/docs/file.txt?versionId=abc123"
```

**Response**
- 200 OK: Object content
- 404 Not Found: Object not found

### Delete Object
```http
DELETE /{bucket}/{key}
```

**Response**
- 204 No Content: Success
- 404 Not Found: Object not found

## Multipart Upload

### Initiate Upload
```http
POST /{bucket}/{key}?uploads
```

**Response**
```xml
<InitiateMultipartUploadResult>
  <Bucket>my-bucket</Bucket>
  <Key>large-file.zip</Key>
  <UploadId>ABC123</UploadId>
</InitiateMultipartUploadResult>
```

### Upload Part
```http
PUT /{bucket}/{key}?partNumber=1&uploadId=ABC123
```

**Response**
```xml
<Part>
  <PartNumber>1</PartNumber>
  <ETag>"d41d8cd98f00b204e9800998ecf8427e"</ETag>
</Part>
```

### Complete Upload
```http
POST /{bucket}/{key}?uploadId=ABC123
```

**Request Body**
```xml
<CompleteMultipartUpload>
  <Part>
    <PartNumber>1</PartNumber>
    <ETag>"d41d8cd98f00b204e9800998ecf8427e"</ETag>
  </Part>
</CompleteMultipartUpload>
```

**Response**
```xml
<CompleteMultipartUploadResult>
  <Location>http://localhost:8080/s3/my-bucket/large-file.zip</Location>
  <ETag>"final-etag"</ETag>
</CompleteMultipartUploadResult>
```

## Error Responses

**Format**
```xml
<Error>
  <Code>NoSuchKey</Code>
  <Message>The specified key does not exist</Message>
  <Resource>/my-bucket/missing-file.txt</Resource>
  <RequestId>ABC123</RequestId>
</Error>
```

**Common Errors**
- 403 Forbidden: Invalid credentials
- 404 Not Found: Bucket/Object not found
- 500 Internal Server Error: Server error
