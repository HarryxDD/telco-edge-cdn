package wtinylfu

import (
	"hash/fnv"

	"github.com/dgryski/go-tinylfu"
)

// Note: We use TinyLFU only for admission control (frequency tracking)
// Actual data storage is in BadgerDB
type WTinyLFUCache struct {
	cache *tinylfu.T[string, struct{}]
}

func hashString(s string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(s))
	return h.Sum64()
}

// capacity: number of items (not bytes)
// samples: number of samples before resetting (typically capacity * 10)
func NewWTinyLFUCache(capacity int, onEvict func(key string)) *WTinyLFUCache {
	cacheOptions := []tinylfu.Option[string, struct{}]{}
	if onEvict != nil {
		cacheOptions = append(cacheOptions, tinylfu.OnEvict(func(key string, _ struct{}) {
			onEvict(key)
		}))
	}

	return &WTinyLFUCache{
		cache: tinylfu.New[string, struct{}](
			capacity,    // max number of items to track
			capacity*10, // samples: reset frequency counters after this many operations
			hashString,  // hash function for string keys
			cacheOptions...,
		),
	}
}

func (w *WTinyLFUCache) Add(key string) {
	w.cache.Add(key, struct{}{})
}

// This updates the frequency counter in TinyLFU
func (w *WTinyLFUCache) Access(key string) {
	w.cache.Get(key)
}
