package storage

type QuotaManager struct {
	limits map[string]int64 // userID -> bytes
	usage  map[string]int64
}

func NewQuotaManager() *QuotaManager {
	return &QuotaManager{
		limits: make(map[string]int64),
		usage:  make(map[string]int64),
	}
}

func (q *QuotaManager) CheckQuota(userID string, size int64) bool {
	return q.usage[userID]+size <= q.limits[userID]
}

func (q *QuotaManager) UpdateUsage(userID string, delta int64) {
	q.usage[userID] += delta
}
