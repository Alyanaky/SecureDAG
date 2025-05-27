package dag

import (
    "bytes"
    "crypto/sha256"
)

type MerkleNode struct {
    Hash  []byte
    Left  *MerkleNode
    Right *MerkleNode
}

func NewMerkleNode(data []byte, left, right *MerkleNode) *MerkleNode {
    var hash [32]byte
    if left == nil && right == nil {
        hash = sha256.Sum256(data)
    } else {
        var buf bytes.Buffer
        if left != nil {
            buf.Write(left.Hash)
        }
        if right != nil {
            buf.Write(right.Hash)
        }
        hash = sha256.Sum256(buf.Bytes())
    }
    return &MerkleNode{
        Hash:  hash[:],
        Left:  left,
        Right: right,
    }
}
