package cache

import (
	"context"
	"encoding/json"
	"time"

	"github.com/allegro/bigcache/v3"
	"github.com/vmihailenco/msgpack/v5"
)

// L1Cache provides ultra-fast embedded caching with zero GC overhead
type L1Cache struct {
	cache *bigcache.BigCache
}

// NewL1Cache creates a new embedded cache optimized for financial data
func NewL1Cache() (*L1Cache, error) {
	config := bigcache.Config{
		Shards:             1024,           // Power of 2 for optimal sharding
		LifeWindow:         10 * time.Minute, // Market data TTL
		CleanWindow:        5 * time.Minute,  // Cleanup interval
		MaxEntriesInWindow: 1000 * 10 * 60,   // 10 minutes worth of data
		MaxEntrySize:       500,              // Bytes per entry
		HardMaxCacheSize:   512,              // MB
		Verbose:            false,
	}

	cache, err := bigcache.New(context.Background(), config)
	if err != nil {
		return nil, err
	}

	return &L1Cache{cache: cache}, nil
}

// Set stores data with MessagePack serialization (2x faster than JSON)
func (c *L1Cache) Set(key string, value interface{}) error {
	data, err := msgpack.Marshal(value)
	if err != nil {
		return err
	}
	return c.cache.Set(key, data)
}

// Get retrieves and deserializes data
func (c *L1Cache) Get(key string, dest interface{}) error {
	data, err := c.cache.Get(key)
	if err != nil {
		return err
	}
	return msgpack.Unmarshal(data, dest)
}

// SetJSON provides JSON fallback for compatibility
func (c *L1Cache) SetJSON(key string, value interface{}) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.cache.Set(key, data)
}

// GetJSON retrieves JSON data for compatibility
func (c *L1Cache) GetJSON(key string, dest interface{}) error {
	data, err := c.cache.Get(key)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dest)
}

// Delete removes an entry
func (c *L1Cache) Delete(key string) error {
	return c.cache.Delete(key)
}

// Stats returns cache statistics
func (c *L1Cache) Stats() bigcache.Stats {
	return c.cache.Stats()
}

// Close cleans up resources
func (c *L1Cache) Close() error {
	return c.cache.Close()
}