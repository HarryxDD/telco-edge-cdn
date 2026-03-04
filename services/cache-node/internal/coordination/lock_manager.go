package coordination

import (
	"fmt"
	"sync"
	"time"
)

// FetchLock represents a lock for fetching from origin
type FetchLock struct {
	SegmentID    string
	FetchingNode string
	AcquiredAt   time.Time
	ExpiresAt    time.Time
}

// LockManager manages fetch locks to prevent duplicate origin requests
type LockManager struct {
	mu    sync.RWMutex
	locks map[string]*FetchLock
}

func NewLockManager() *LockManager {
	return &LockManager{
		locks: make(map[string]*FetchLock),
	}
}

// RequestFetchLock attempts to acquire a lock for fetching a segment
func (lm *LockManager) RequestFetchLock(segmentID, nodeID string, timeout time.Duration) (granted bool, fetchingNode string) {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	// Check if lock already exists
	if lock, exists := lm.locks[segmentID]; exists {
		// Check if expired
		if time.Now().After(lock.ExpiresAt) {
			delete(lm.locks, segmentID)
		} else {
			// Someone else is fetching
			return false, lock.FetchingNode
		}
	}

	// Grant lock
	lm.locks[segmentID] = &FetchLock{
		SegmentID:    segmentID,
		FetchingNode: nodeID,
		AcquiredAt:   time.Now(),
		ExpiresAt:    time.Now().Add(timeout),
	}

	return true, nodeID
}

// ReleaseFetchLock releases a fetch lock after completion
func (lm *LockManager) ReleaseFetchLock(segmentID string, nodeID string) error {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	lock, exists := lm.locks[segmentID]
	if !exists {
		return fmt.Errorf("lock not found for segment %s", segmentID)
	}

	if lock.FetchingNode != nodeID {
		return fmt.Errorf("lock owned by %s, not %s", lock.FetchingNode, nodeID)
	}

	delete(lm.locks, segmentID)
	return nil
}

// CleanupExpiredLocks removes expired locks
func (lm *LockManager) CleanupExpiredLocks() {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	now := time.Now()
	for segmentID, lock := range lm.locks {
		if now.After(lock.ExpiresAt) {
			delete(lm.locks, segmentID)
		}
	}
}
