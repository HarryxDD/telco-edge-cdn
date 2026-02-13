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
	mu      sync.RWMutex
	locks   map[string]*FetchLock
	waiters map[string][]chan struct{}
}

func NewLockManager() *LockManager {
	return &LockManager{
		locks:   make(map[string]*FetchLock),
		waiters: make(map[string][]chan struct{}),
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

	// Notify waiters
	if waiters, exists := lm.waiters[segmentID]; exists {
		for _, ch := range waiters {
			close(ch)
		}
		delete(lm.waiters, segmentID)
	}

	return nil
}

// WaitForSegment blocks until segment fetch completes or timeout
func (lm *LockManager) WaitForSegment(segmentID string, timeout time.Duration) bool {
	lm.mu.Lock()

	// Check if lock exists
	if _, exists := lm.locks[segmentID]; !exists {
		lm.mu.Unlock()
		return true
	}

	ch := make(chan struct{})
	if _, exists := lm.waiters[segmentID]; !exists {
		lm.waiters[segmentID] = make([]chan struct{}, 0)
	}
	lm.waiters[segmentID] = append(lm.waiters[segmentID], ch)
	lm.mu.Unlock()

	// Wait for notification or timeout
	select {
	case <-ch:
		return true
	case <-time.After(timeout):
		return false
	}
}

// CleanupExpiredLocks removes expired locks
func (lm *LockManager) CleanupExpiredLocks() {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	now := time.Now()
	for segmentID, lock := range lm.locks {
		if now.After(lock.ExpiresAt) {
			delete(lm.locks, segmentID)

			// Notify waiters
			if waiters, exists := lm.waiters[segmentID]; exists {
				for _, ch := range waiters {
					close(ch)
				}
				delete(lm.waiters, segmentID)
			}
		}
	}
}
