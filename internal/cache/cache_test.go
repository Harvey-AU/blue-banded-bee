package cache

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewInMemoryCache(t *testing.T) {
	cache := NewInMemoryCache()

	assert.NotNil(t, cache)
	assert.NotNil(t, cache.items)
	assert.Empty(t, cache.items)
}

func TestInMemoryCache_GetSet(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value interface{}
	}{
		{
			name:  "string_value",
			key:   "test-key",
			value: "test-value",
		},
		{
			name:  "int_value",
			key:   "count",
			value: 42,
		},
		{
			name: "struct_value",
			key:  "user",
			value: struct {
				Name string
				Age  int
			}{Name: "John", Age: 30},
		},
		{
			name:  "nil_value",
			key:   "nil-key",
			value: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := NewInMemoryCache()

			// Test Get on empty cache
			val, found := cache.Get(tt.key)
			assert.False(t, found)
			assert.Nil(t, val)

			// Test Set
			cache.Set(tt.key, tt.value)

			// Test Get after Set
			val, found = cache.Get(tt.key)
			assert.True(t, found)
			assert.Equal(t, tt.value, val)

			// Test overwrite
			newValue := "overwritten"
			cache.Set(tt.key, newValue)
			val, found = cache.Get(tt.key)
			assert.True(t, found)
			assert.Equal(t, newValue, val)
		})
	}
}

func TestInMemoryCache_Delete(t *testing.T) {
	cache := NewInMemoryCache()

	// Set some values
	cache.Set("key1", "value1")
	cache.Set("key2", "value2")
	cache.Set("key3", "value3")

	// Verify they exist
	val, found := cache.Get("key2")
	require.True(t, found)
	assert.Equal(t, "value2", val)

	// Delete key2
	cache.Delete("key2")

	// Verify key2 is gone
	val, found = cache.Get("key2")
	assert.False(t, found)
	assert.Nil(t, val)

	// Verify other keys still exist
	val, found = cache.Get("key1")
	assert.True(t, found)
	assert.Equal(t, "value1", val)

	val, found = cache.Get("key3")
	assert.True(t, found)
	assert.Equal(t, "value3", val)

	// Delete non-existent key (should not panic)
	cache.Delete("non-existent")
}

func TestInMemoryCache_Concurrent(t *testing.T) {
	cache := NewInMemoryCache()
	const numGoroutines = 100
	const numOperations = 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines * 3) // 3 types of operations

	// Concurrent Sets
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				key := "key" + string(rune('0'+id%10))
				cache.Set(key, id*1000+j)
			}
		}(i)
	}

	// Concurrent Gets
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				key := "key" + string(rune('0'+id%10))
				cache.Get(key)
			}
		}(i)
	}

	// Concurrent Deletes
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				if j%10 == 0 { // Delete every 10th operation
					key := "key" + string(rune('0'+id%10))
					cache.Delete(key)
				}
			}
		}(i)
	}

	wg.Wait()

	// Cache should still be functional after concurrent operations
	cache.Set("final", "test")
	val, found := cache.Get("final")
	assert.True(t, found)
	assert.Equal(t, "test", val)
}

func TestInMemoryCache_MultipleKeys(t *testing.T) {
	cache := NewInMemoryCache()

	// Add multiple keys
	keys := []string{"alpha", "beta", "gamma", "delta", "epsilon"}
	for i, key := range keys {
		cache.Set(key, i*10)
	}

	// Verify all keys exist with correct values
	for i, key := range keys {
		val, found := cache.Get(key)
		assert.True(t, found)
		assert.Equal(t, i*10, val)
	}

	// Delete odd-indexed keys
	for i, key := range keys {
		if i%2 == 1 {
			cache.Delete(key)
		}
	}

	// Verify deletion
	for i, key := range keys {
		val, found := cache.Get(key)
		if i%2 == 0 {
			assert.True(t, found)
			assert.Equal(t, i*10, val)
		} else {
			assert.False(t, found)
			assert.Nil(t, val)
		}
	}
}

func BenchmarkInMemoryCache_Set(b *testing.B) {
	cache := NewInMemoryCache()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		cache.Set("bench-key", i)
	}
}

func BenchmarkInMemoryCache_Get(b *testing.B) {
	cache := NewInMemoryCache()
	cache.Set("bench-key", "bench-value")
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		cache.Get("bench-key")
	}
}

func BenchmarkInMemoryCache_Delete(b *testing.B) {
	cache := NewInMemoryCache()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		cache.Set("bench-key", i)
		cache.Delete("bench-key")
	}
}
