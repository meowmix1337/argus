package service

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestCacheService_SetAndGet(t *testing.T) {
	c := NewCacheService()
	c.Set("key", "value", time.Minute)

	v, ok := c.Get("key")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if v.(string) != "value" {
		t.Errorf("expected %q, got %q", "value", v)
	}
}

func TestCacheService_Miss(t *testing.T) {
	c := NewCacheService()
	_, ok := c.Get("missing")
	if ok {
		t.Error("expected cache miss for unknown key")
	}
}

func TestCacheService_TTLExpiry(t *testing.T) {
	c := NewCacheService()
	c.Set("key", "value", 50*time.Millisecond)
	time.Sleep(150 * time.Millisecond)

	_, ok := c.Get("key")
	if ok {
		t.Error("expected expired entry to be a cache miss")
	}
}

func TestCacheService_Delete(t *testing.T) {
	c := NewCacheService()
	c.Set("key", "value", time.Minute)
	c.Delete("key")

	_, ok := c.Get("key")
	if ok {
		t.Error("expected cache miss after Delete")
	}
}

func TestCacheService_Overwrite(t *testing.T) {
	c := NewCacheService()
	c.Set("key", "first", time.Minute)
	c.Set("key", "second", time.Minute)

	v, ok := c.Get("key")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if v.(string) != "second" {
		t.Errorf("expected %q, got %q", "second", v)
	}
}

// TestCacheService_ConcurrentAccess validates safety under the -race detector.
func TestCacheService_ConcurrentAccess(t *testing.T) {
	c := NewCacheService()
	var wg sync.WaitGroup
	for n := range 100 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			key := fmt.Sprintf("key-%d", n)
			c.Set(key, n, time.Minute)
			_, _ = c.Get(key)
			c.Delete(key)
		}(n)
	}
	wg.Wait()
}
