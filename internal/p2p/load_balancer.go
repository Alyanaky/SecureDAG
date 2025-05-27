package p2p

type LoadBalancer struct {
    nodes []string
}

func NewLoadBalancer(nodes []string) *LoadBalancer {
    return &LoadBalancer{nodes: nodes}
}

func (lb *LoadBalancer) SelectNode() string {
    if len(lb.nodes) == 0 {
        return ""
    }
    return lb.nodes[0]
}
