package cache

import "sync"

// InMemoryCache is a simple, concurrent-safe in-memory key-value store.
type InMemoryCache struct {
	mu    sync.RWMutex
	items map[string]any
}

// NewInMemoryCache creates and returns a new InMemoryCache.
func NewInMemoryCache() *InMemoryCache {
	return &InMemoryCache{
		items: make(map[string]any),
	}
}

// Get retrieves a value from the cache.
// It returns the value and true if the key exists, otherwise nil and false.
func (c *InMemoryCache) Get(key string) (any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	item, found := c.items[key]
	return item, found
}

// Set adds or updates a value in the cache.
func (c *InMemoryCache) Set(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[key] = value
}

// Delete removes a value from the cache.
func (c *InMemoryCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, key)
}
