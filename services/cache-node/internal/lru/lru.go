package lru

import (
	"log"
	"sync"
)

type LRUCache struct {
	mu          sync.RWMutex
	capacity    int64
	currentSize int64
	items       map[string]*lruNode
	head        *lruNode
	tail        *lruNode
}

type lruNode struct {
	key  string
	size int64
	prev *lruNode
	next *lruNode
}

func NewLRUCache(capacity int64) *LRUCache {
	head := &lruNode{}
	tail := &lruNode{}
	head.next = tail
	tail.prev = head

	return &LRUCache{
		capacity: capacity,
		items:    make(map[string]*lruNode),
		head:     head,
		tail:     tail,
	}
}

func (c *LRUCache) moveToFront(node *lruNode) {
	c.removeNode(node)
	c.addToFront(node)
}

func (c *LRUCache) removeNode(node *lruNode) {
	node.prev.next = node.next
	node.next.prev = node.prev
}

func (c *LRUCache) addToFront(node *lruNode) {
	node.next = c.head.next
	node.prev = c.head
	c.head.next.prev = node
	c.head.next = node
}

func (c *LRUCache) removeLRU() string {
	if c.tail.prev == c.head {
		return ""
	}

	lru := c.tail.prev
	c.removeNode(lru)
	delete(c.items, lru.key)
	c.currentSize -= lru.size
	return lru.key
}

func (c *LRUCache) Access(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if node, exists := c.items[key]; exists {
		c.moveToFront(node)
	}
}

func (c *LRUCache) Insert(key string, size int64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Evict until we have space
	for c.currentSize+size > c.capacity && len(c.items) > 0 {
		evicted := c.removeLRU()
		log.Printf("Evicted: %s", evicted)
	}

	node := &lruNode{key: key, size: size}
	c.addToFront(node)
	c.items[key] = node
	c.currentSize += size
}
