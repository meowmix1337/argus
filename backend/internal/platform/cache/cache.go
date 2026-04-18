package cache

import (
	"sync"
	"time"
)

type cacheEntry struct {
	value     any
	expiresAt time.Time
}

// CacheService is a generic in-memory TTL cache backed by sync.Map.
type CacheService struct {
	data sync.Map
}

// NewCacheService creates a new CacheService and starts the background eviction goroutine.
func NewCacheService() *CacheService {
	c := &CacheService{}
	go c.evict()
	return c
}

// Get returns the cached value for key if it exists and has not expired.
func (c *CacheService) Get(key string) (any, bool) {
	v, ok := c.data.Load(key)
	if !ok {
		return nil, false
	}
	entry := v.(cacheEntry)
	if time.Now().After(entry.expiresAt) {
		c.data.Delete(key)
		return nil, false
	}
	return entry.value, true
}

// Set stores value in the cache with the given TTL.
func (c *CacheService) Set(key string, value any, ttl time.Duration) {
	c.data.Store(key, cacheEntry{
		value:     value,
		expiresAt: time.Now().Add(ttl),
	})
}

// Delete removes the cache entry for key immediately.
func (c *CacheService) Delete(key string) {
	c.data.Delete(key)
}

// cacheEvictInterval is how often the background eviction loop runs.
const cacheEvictInterval = 60 * time.Second

// evict runs every cacheEvictInterval and removes expired entries.
func (c *CacheService) evict() {
	ticker := time.NewTicker(cacheEvictInterval)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		c.data.Range(func(k, v any) bool {
			if now.After(v.(cacheEntry).expiresAt) {
				c.data.Delete(k)
			}
			return true
		})
	}
}
