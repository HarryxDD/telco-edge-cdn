package service

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/HarryxDD/telco-edge-cdn/cache-node/internal/wtinylfu"
	badger "github.com/dgraph-io/badger/v4"
)

type EdgeCache struct {
	db         *badger.DB
	eviction   *wtinylfu.WTinyLFUCache
	OriginURL  string
	NodeID     string
	httpClient *http.Client
}

func NewEdgeCache(dbPath, originURL, nodeID string, cacheCapacity int) (*EdgeCache, error) {
	opts := badger.DefaultOptions(dbPath)
	opts.Logger = nil

	db, err := badger.Open(opts)
	if err != nil {
		return nil, err
	}

	onEvict := func(key string) {
		err := db.Update(func(txn *badger.Txn) error {
			return txn.Delete([]byte(key))
		})
		if err != nil {
			log.Printf("[%s] ERROR evicted item deletion failed: %v", nodeID, err)
		} else {
			log.Printf("[%s] EVICTED: %s", nodeID, key)
		}
	}

	return &EdgeCache{
		db:        db,
		eviction:  wtinylfu.NewWTinyLFUCache(cacheCapacity, onEvict),
		OriginURL: originURL,
		NodeID:    nodeID,
		httpClient: &http.Client{
			Timeout: 30 * time.Second, // Increased for high-latency demos
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
		e.eviction.Access(key)
		log.Printf("[%s] CACHE HIT: %s", e.NodeID, key)
		return data, true
	}

	log.Printf("[%s] CACHE MISS: %s", e.NodeID, key)
	return nil, false
}

func (e *EdgeCache) Put(key string, data []byte) error {
	err := e.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(key), data)
	})

	if err == nil {
		e.eviction.Add(key)
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
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (e *EdgeCache) Close() error {
	return e.db.Close()
}
