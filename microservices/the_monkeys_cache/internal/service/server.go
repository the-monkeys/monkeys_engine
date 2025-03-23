package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

type CacheItem struct {
	Value      []byte
	Expiration int64
}

type CacheServer struct {
	items sync.Map
	log   *logrus.Logger
}

func NewCacheServer(log *logrus.Logger) *CacheServer {
	cache := &CacheServer{
		log: log,
	}

	go cache.cleanupLoop()

	return cache
}

func (c *CacheServer) Set(ctx context.Context, key string, value any, expiration time.Duration) error {
	bytes, err := json.Marshal(value)
	if err != nil {
		return err
	}

	var exp int64
	if expiration > 0 {
		exp = time.Now().Add(expiration).UnixNano()
	}

	fmt.Printf("key: %v\n", key)

	c.items.Store(key, CacheItem{
		Value:      bytes,
		Expiration: exp,
	})

	return nil
}

func (c *CacheServer) Get(ctx context.Context, key string) (any, error) {
	fmt.Printf("key: %v\n", key)

	item, exists := c.items.Load(key)
	if !exists {
		return nil, errors.New("key not found")
	}

	cacheItem := item.(CacheItem)
	if cacheItem.Expiration > 0 && time.Now().UnixNano() > cacheItem.Expiration {
		c.items.Delete(key)
		return nil, errors.New("item expired")
	}

	var value any
	if err := json.Unmarshal(cacheItem.Value, &value); err != nil {
		return nil, err
	}

	return value, nil
}

func (c *CacheServer) Delete(ctx context.Context, key string) error {
	c.items.Delete(key)
	return nil
}

func (c *CacheServer) Clear(ctx context.Context) error {
	c.items.Range(func(key, value any) bool {
		c.items.Delete(key)
		return true
	})
	return nil
}

func (c *CacheServer) cleanupLoop() {
	ticker := time.NewTicker(time.Minute)
	for range ticker.C {
		c.cleanup()
	}
}

func (c *CacheServer) cleanup() {
	now := time.Now().UnixNano()
	c.items.Range(func(key, value any) bool {
		item := value.(CacheItem)
		if item.Expiration > 0 && now > item.Expiration {
			c.items.Delete(key)
		}
		return true
	})
}

func (c *CacheServer) GetStats(ctx context.Context) (map[string]any, error) {
	now := time.Now().UnixNano()
	expired := 0
	total := 0

	c.items.Range(func(key, value any) bool {
		total++
		item := value.(CacheItem)
		if item.Expiration > 0 && now > item.Expiration {
			expired++
		}
		return true
	})

	return map[string]any{
		"total_items": total,
		"expired":     expired,
		"active":      total - expired,
	}, nil
}
