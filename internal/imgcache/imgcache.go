// Package imgcache keeps recently served images in memory so repeated icon
// requests never touch SQLite. The cache is a byte-bounded LRU.
package imgcache

import (
	"container/list"
	"sync"
)

// Entry is one cached image.
type Entry struct {
	ID   string
	Mime string
	Data []byte
	ETag string
	Size int64
}

// Cache is a concurrency safe LRU with a byte budget.
type Cache struct {
	mu       sync.Mutex
	ll       *list.List
	items    map[string]*list.Element
	bytes    int64
	maxBytes int64
	hits     int64
	misses   int64
}

type node struct {
	key   string
	entry *Entry
}

// New creates a cache with a byte budget. A budget <= 0 disables caching.
func New(maxBytes int64) *Cache {
	return &Cache{
		ll:       list.New(),
		items:    make(map[string]*list.Element),
		maxBytes: maxBytes,
	}
}

// Get returns a cached entry and promotes it to the front.
func (c *Cache) Get(key string) (*Entry, bool) {
	if c == nil || c.maxBytes <= 0 {
		return nil, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[key]; ok {
		c.ll.MoveToFront(el)
		c.hits++
		return el.Value.(*node).entry, true
	}
	c.misses++
	return nil, false
}

// Put inserts an entry, evicting least recently used entries as needed.
func (c *Cache) Put(e *Entry) {
	if c == nil || c.maxBytes <= 0 || e == nil || int64(len(e.Data)) > c.maxBytes {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[e.ID]; ok {
		old := el.Value.(*node)
		c.bytes -= int64(len(old.entry.Data))
		old.entry = e
		c.bytes += int64(len(e.Data))
		c.ll.MoveToFront(el)
		return
	}
	el := c.ll.PushFront(&node{key: e.ID, entry: e})
	c.items[e.ID] = el
	c.bytes += int64(len(e.Data))
	for c.bytes > c.maxBytes {
		back := c.ll.Back()
		if back == nil {
			break
		}
		n := back.Value.(*node)
		delete(c.items, n.key)
		c.bytes -= int64(len(n.entry.Data))
		c.ll.Remove(back)
	}
}

// Delete drops one entry.
func (c *Cache) Delete(key string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[key]; ok {
		n := el.Value.(*node)
		c.bytes -= int64(len(n.entry.Data))
		delete(c.items, n.key)
		c.ll.Remove(el)
	}
}

// Stats reports cache metrics for /api/healthz.
func (c *Cache) Stats() (entries int, bytes, hits, misses int64) {
	if c == nil {
		return 0, 0, 0, 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.ll.Len(), c.bytes, c.hits, c.misses
}
