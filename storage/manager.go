package storage

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
)

type Block struct {
	ID   []byte
	Data []byte
}

type Storage struct {
	blocks map[string][]byte
}

func NewStorage() *Storage {
	return &Storage{
		blocks: make(map[string][]byte),
	}
}

func (s *Storage) SplitAndStore(r io.Reader, chunkSize int) ([]string, error) {
	var hashes []string
	buf := make([]byte, chunkSize)
	
	for {
		n, err := r.Read(buf)
		if err != nil && err != io.EOF {
			return nil, err
		}
		if n == 0 {
			break
		}

		chunk := make([]byte, n)
		copy(chunk, buf[:n])
		
		hash := sha256.Sum256(chunk)
		hashStr := fmt.Sprintf("%x", hash)
		
		s.blocks[hashStr] = chunk
		hashes = append(hashes, hashStr)
		
		if err == io.EOF {
			break
		}
	}
	
	return hashes, nil
}

func (s *Storage) GetBlock(hash string) ([]byte, bool) {
	data, exists := s.blocks[hash]
	return data, exists
}
