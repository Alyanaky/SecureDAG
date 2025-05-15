package storage

import (
	"bytes"
	"encoding/json"
	
	"github.com/Alyanaky/SecureDAG/internal/dag"
)

func (s *BadgerStore) BuildDAG(hashes []string) (*dag.Node, error) {
	nodes := make([]*dag.Node, len(hashes))
	
	for i, hash := range hashes {
		data, err := s.GetBlock(hash)
		if err != nil {
			return nil, err
		}
		nodes[i] = dag.NewMerkleNode(nil, data)
	}

	root := buildMerkleTree(nodes)
	return root, nil
}

func buildMerkleTree(nodes []*dag.Node) *dag.Node {
	if len(nodes) == 1 {
		return nodes[0]
	}

	var parents []*dag.Node
	for i := 0; i < len(nodes); i += 2 {
		if i+1 >= len(nodes) {
			parents = append(parents, nodes[i])
			continue
		}
		parent := dag.NewMerkleNode([]*dag.Node{nodes[i], nodes[i+1]}, nil)
		parents = append(parents, parent)
	}
	return buildMerkleTree(parents)
}

func (s *BadgerStore) StoreDAG(root *dag.Node) (string, error) {
	buf := new(bytes.Buffer)
	enc := json.NewEncoder(buf)
	if err := enc.Encode(root); err != nil {
		return "", err
	}
	return s.PutBlock(buf.Bytes())
}
