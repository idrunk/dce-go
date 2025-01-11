package util

import (
	"slices"
	"sync"
)

type Config struct {
	scalars sync.Map
	vectors sync.Map
}

func NewConfig() *Config {
	return &Config{
		scalars: NewStruct[sync.Map](),
		vectors: NewStruct[sync.Map](),
	}
}

func (c *Config) SetScalar(key string, value any) *Config {
	c.scalars.Store(key, value)
	return c
}

func (c *Config) Scalar(key string) (any, bool) {
	v, ok := c.scalars.Load(key)
	return v, ok
}

func (c *Config) SetVector(key string, value []any) *Config {
	c.vectors.Store(key, value)
	return c
}

func (c *Config) Vector(key string) []any {
	if v, ok := c.vectors.Load(key); ok {
		return v.([]any)
	}
	return nil
}

func (c *Config) VecPut(key string, value any) *Config {
	if vec, ok := c.vectors.LoadOrStore(key, []any{}); ok {
		vector := vec.([]any)
		c.vectors.Store(key, append(vector, value))
	}
	return c
}

func (c *Config) VecPutIfAbsent(key string, value any) []any {
	var vector []any
	if vec, ok := c.vectors.Load(key); ok {
		vector = vec.([]any)
	}
	if !slices.Contains(vector, value) {
		vector = append(vector, value)
		c.vectors.Store(key, vector)
	}
	return vector
}
