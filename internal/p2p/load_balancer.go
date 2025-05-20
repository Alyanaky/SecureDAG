package p2p

import (
    "context"
    "sort"
    "sync"
    "time"

    "github.com/libp2p/go-libp2p/core/host"
)

type NodeLoad struct {
    NodeID  string
    Load    int
    CPU     float64
    Mem     float64
    Updated time.Time
}

type LoadBalancer struct {
    mu    sync.RWMutex
    nodes map[string]*NodeLoad
}

func NewLoadBalancer() *LoadBalancer {
    return &LoadBalancer{
        nodes: make(map[string]*NodeLoad),
    }
}

func (lb *LoadBalancer) UpdateMetrics(nodeID string, load int, cpu, mem float64) {
    lb.mu.Lock()
    defer lb.mu.Unlock()
    
    if node, exists := lb.nodes[nodeID]; exists {
        node.Load = load
        node.CPU = cpu
        node.Mem = mem
        node.Updated = time.Now()
    } else {
        lb.nodes[nodeID] = &NodeLoad{
            NodeID:  nodeID,
            Load:    load,
            CPU:     cpu,
            Mem:     mem,
            Updated: time.Now(),
        }
    }
}

func (lb *LoadBalancer) SelectNodes(count int) []string {
    lb.mu.RLock()
    defer lb.mu.RUnlock()
    
    activeNodes := make([]*NodeLoad, 0)
    for _, n := range lb.nodes {
        if time.Since(n.Updated) < 2*time.Minute {
            activeNodes = append(activeNodes, n)
        }
    }
    
    sort.Slice(activeNodes, func(i, j int) bool {
        scoreI := 0.4*float64(activeNodes[i].Load) + 0.3*activeNodes[i].CPU + 0.3*activeNodes[i].Mem
        scoreJ := 0.4*float64(activeNodes[j].Load) + 0.3*activeNodes[j].CPU + 0.3*activeNodes[j].Mem
        return scoreI < scoreJ
    })
    
    if len(activeNodes) < count {
        count = len(activeNodes)
    }
    
    selected := make([]string, count)
    for i := 0; i < count; i++ {
        selected[i] = activeNodes[i].NodeID
    }
    return selected
}
