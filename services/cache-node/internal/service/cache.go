package service

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	. "github.com/HarryxDD/telco-edge-cdn/cache-node/internal/lru"
	badger "github.com/dgraph-io/badger/v4"
)

type EdgeCache struct {
	db         *badger.DB
	lru        *LRUCache
	OriginURL  string
	NodeID     string
	httpClient *http.Client
}

func NewEdgeCache(dbPath, originURL, nodeID string, cacheCapacity int64) (*EdgeCache, error) {
	opts := badger.DefaultOptions(dbPath)
	opts.Logger = nil

	db, err := badger.Open(opts)
	if err != nil {
		return nil, err
	}

	return &EdgeCache{
		db:        db,
		lru:       NewLRUCache(cacheCapacity),
		OriginURL: originURL,
		NodeID:    nodeID,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}, nil
}

func (e *EdgeCache) Get(key string) ([]byte, bool) {
	var data []byte
	err := e.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}
		data, err = item.ValueCopy(nil)
		return err
	})

	if err == nil {
		e.lru.Access(key)
		// TODO: Log cache hit here
		return data, true
	}

	// TODO: Log cache missed here
	return nil, false
}

func (e *EdgeCache) Put(key string, data []byte) error {
	err := e.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(key), data)
	})

	if err == nil {
		e.lru.Insert(key, int64(len(data)))
	}

	return err
}

func (e *EdgeCache) FetchFromOrigin(path string) ([]byte, error) {
	url := e.OriginURL + path
	log.Printf("Fetching from origin: %s", url)

	resp, err := e.httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Origin returned %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	return data, err
}
