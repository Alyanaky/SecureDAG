# SecureDAG

SecureDAG is a distributed, secure storage system that leverages Directed Acyclic Graphs (DAGs) for data organization, BadgerDB for persistent storage, and an S3-compatible API for object storage. It incorporates cryptographic key management, quota enforcement, and self-healing mechanisms to ensure data integrity and availability.

## Features
- **S3-Compatible API**: Supports standard S3 operations (`PUT`, `GET`, `DELETE`) for object storage.
- **Distributed Storage**: Uses BadgerDB as the backend with replication via a P2P DHT.
- **Cryptographic Security**: AES encryption for data and RSA key rotation for secure key management.
- **Quota Management**: Enforces storage limits per bucket.
- **Self-Healing**: Automatically repairs data inconsistencies in the storage layer.
- **Integration Testing**: Includes tests for S3 operations and quota management.

## Prerequisites
- Go 1.24 or later
- PostgreSQL (for metadata storage)
- BadgerDB dependencies
- AWS SDK for Go v2 (`github.com/aws/aws-sdk-go-v2`)

## Installation

1. **Clone the repository**:
   ```bash
   git clone https://github.com/Alyanaky/SecureDAG.git
   cd SecureDAG

    Install dependencies:
    bash

go mod tidy
Set up PostgreSQL:

    Create a database named securedag:
    sql

CREATE DATABASE securedag;
Update the connection string in cmd/api/server.go if needed:
go

    connStr := "postgres://user:password@localhost:5432/securedag?sslmode=disable"

Run the API server:
bash

    go run cmd/api/server.go

Usage
Running the API Server

The API server exposes S3-compatible endpoints at http://localhost:8080.

    Create a bucket:
    bash

curl -X PUT -H "Authorization: <JWT_TOKEN>" http://localhost:8080/s3/testbucket
Upload an object:
bash
curl -X PUT -H "Authorization: <JWT_TOKEN>" -H "Content-Type: text/plain" --data "test data" http://localhost:8080/s3/testbucket/testkey
Retrieve an object:
bash
curl -X GET -H "Authorization: <JWT_TOKEN>" http://localhost:8080/s3/testbucket/testkey
Delete an object:
bash

    curl -X DELETE -H "Authorization: <JWT_TOKEN>" http://localhost:8080/s3/testbucket/testkey

Generating JWT Tokens

Tokens are generated using RSA-based JWTs. For testing, a temporary key is set in integration tests. In production, ensure proper key management via internal/auth/jwt.go.
