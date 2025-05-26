package storage

type QuotaManager struct {
	store *BadgerStore
}

func NewQuotaManager(store *BadgerStore) *QuotaManager {
	return &QuotaManager{store: store}
}

func (q *QuotaManager) CheckQuota(userID string, size int64) (bool, error) {
	limit, err := q.store.GetQuota(userID)
	if err != nil {
		return false, err
	}
	usage, err := q.store.GetUsage(userID)
	if err != nil {
		return false, err
	}
	return usage + size <= limit, nil
}

func (q *QuotaManager) UpdateUsage(userID string, delta int64) error {
	usage, err := q.store.GetUsage(userID)
	if err != nil {
		return err
	}
	usage += delta
	return q.store.SetUsage(userID, usage)
}
