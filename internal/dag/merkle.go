package dag

import (
	"crypto/sha256"
	"errors"
)

type Node struct {
	Hash  []byte
	Links []*Node
}

func NewMerkleNode(links []*Node, data []byte) *Node {
	if len(links) == 0 {
		return &Node{Hash: hashData(data)}
	}
	return &Node{Hash: hashLinks(links), Links: links}
}

func hashData(data []byte) []byte {
	h := sha256.Sum256(data)
	return h[:]
}

func hashLinks(links []*Node) []byte {
	h := sha256.New()
	for _, link := range links {
		h.Write(link.Hash)
	}
	return h.Sum(nil)
}

func Verify(root *Node) error {
	for _, link := range root.Links {
		if !bytes.Equal(link.Hash, hashLinks(link.Links)) {
			return errors.New("invalid merkle structure")
		}
		if err := Verify(link); err != nil {
			return err
		}
	}
	return nil
}
