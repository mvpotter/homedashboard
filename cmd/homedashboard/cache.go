package main

import (
	"sync"
	"time"
)

type CachedImage struct {
	mu        sync.RWMutex
	data      []byte
	updatedAt time.Time
}

func (c *CachedImage) Set(data []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data = data
	c.updatedAt = time.Now()
}

func (c *CachedImage) Get() ([]byte, time.Time) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.data, c.updatedAt
}
