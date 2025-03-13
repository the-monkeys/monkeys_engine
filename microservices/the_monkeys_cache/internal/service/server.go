package service

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

type CacheItem struct {
	Value      []byte
	Expiration int64
}

type CacheServer struct {
	mu    sync.RWMutex
	items map[string]CacheItem
	log   *logrus.Logger
}

func NewCacheServer(log *logrus.Logger) *CacheServer {
	cache := &CacheServer{
		items: make(map[string]CacheItem),
		log:   log,
	}

	go cache.cleanupLoop()

	return cache
}

func (c *CacheServer) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	bytes, err := json.Marshal(value)
	if err != nil {
		return err
	}

	var exp int64
	if expiration > 0 {
		exp = time.Now().Add(expiration).UnixNano()
	}

	c.items[key] = CacheItem{
		Value:      bytes,
		Expiration: exp,
	}

	return nil
}

func (c *CacheServer) Get(ctx context.Context, key string) (interface{}, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, exists := c.items[key]
	if !exists {
		return nil, errors.New("key not found")
	}

	if item.Expiration > 0 && time.Now().UnixNano() > item.Expiration {
		delete(c.items, key)
		return nil, errors.New("item expired")
	}

	var value interface{}
	if err := json.Unmarshal(item.Value, &value); err != nil {
		return nil, err
	}

	return value, nil
}

func (c *CacheServer) Delete(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.items, key)
	return nil
}

func (c *CacheServer) Clear(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]CacheItem)
	return nil
}

func (c *CacheServer) cleanupLoop() {
	ticker := time.NewTicker(time.Minute)
	for range ticker.C {
		c.cleanup()
	}
}

func (c *CacheServer) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now().UnixNano()
	for key, item := range c.items {
		if item.Expiration > 0 && now > item.Expiration {
			delete(c.items, key)
		}
	}
}

func (c *CacheServer) GetStats(ctx context.Context) (map[string]interface{}, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	now := time.Now().UnixNano()
	expired := 0
	for _, item := range c.items {
		if item.Expiration > 0 && now > item.Expiration {
			expired++
		}
	}

	return map[string]interface{}{
		"total_items": len(c.items),
		"expired":     expired,
		"active":      len(c.items) - expired,
	}, nil
}
