package lifecycle

import (
	"time"
	"github.com/Alyanaky/SecureDAG/internal/storage"
)

type PolicyAction string
const (
	ActionDelete    PolicyAction = "Delete"
	ActionArchive   PolicyAction = "Archive"
	ActionChangeTier PolicyAction = "ChangeTier"
)

type LifecycleRule struct {
	ID          string
	Description string
	Status      string // Enabled/Disabled
	Prefix      string
	Actions     []PolicyAction
	Conditions  struct {
		AgeDays         int
		Versions        int
		CreatedBefore   time.Time
		StorageClass    string
	}
}

type LifecycleManager struct {
	store storage.Store
	rules map[string]LifecycleRule
}

func NewLifecycleManager(store storage.Store) *LifecycleManager {
	return &LifecycleManager{
		store: store,
		rules: make(map[string]LifecycleRule),
	}
}

func (m *LifecycleManager) AddRule(rule LifecycleRule) {
	m.rules[rule.ID] = rule
}

func (m *LifecycleManager) ProcessBucket(bucket string) {
	for _, rule := range m.rules {
		if rule.Status != "Enabled" {
			continue
		}
		
		m.store.IterateObjects(bucket, rule.Prefix, func(obj storage.ObjectVersion) {
			if m.checkConditions(obj, rule) {
				m.applyActions(obj, rule)
			}
		})
	}
}

func (m *LifecycleManager) checkConditions(obj storage.ObjectVersion, rule LifecycleRule) bool {
	if time.Since(obj.Modified).Hours()/24 < float64(rule.Conditions.AgeDays) {
		return false
	}
	
	if rule.Conditions.Versions > 0 && obj.VersionNumber > rule.Conditions.Versions {
		return true
	}
	
	return true
}

func (m *LifecycleManager) applyActions(obj storage.ObjectVersion, rule LifecycleRule) {
	for _, action := range rule.Actions {
		switch action {
		case ActionDelete:
			m.store.DeleteVersion(obj.Bucket, obj.Key, obj.VersionID)
		case ActionArchive:
			m.store.ChangeStorageClass(obj.Bucket, obj.Key, obj.VersionID, "GLACIER")
		}
	}
}
