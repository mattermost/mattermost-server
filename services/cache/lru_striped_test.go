// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package cache

import (
	"fmt"
	"hash/maphash"
	"testing"

	"github.com/cespare/xxhash/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
)

func makeLRUPredictibleTestData(num int) [][2]string {
	kv := make([][2]string, num)
	for i := 0; i < len(kv); i++ {
		kv[i] = [2]string{
			fmt.Sprintf("%d-key-%d", i, i),
			fmt.Sprintf("%d-val-%d", i, i),
		}
	}
	return kv
}

func TestNewLRUStriped(t *testing.T) {
	scache, err := NewLRUStriped(LRUOptions{StripedBuckets: 3, Size: 20})
	require.NoError(t, err)

	cache := scache.(LRUStriped)

	require.Len(t, cache.buckets, 3)
	assert.Equal(t, 8, cache.buckets[0].size)
	assert.Equal(t, 8, cache.buckets[1].size)
	assert.Equal(t, 8, cache.buckets[2].size)
}

func TestLRUStripedKeyDistribution(t *testing.T) {
	dataset := makeLRUPredictibleTestData(100)

	scache, err := NewLRUStriped(LRUOptions{StripedBuckets: 4, Size: len(dataset)})
	require.NoError(t, err)
	cache := scache.(LRUStriped)
	for _, kv := range dataset {
		require.NoError(t, cache.Set(kv[0], kv[1]))
		var out string
		require.NoError(t, cache.Get(kv[0], &out))
		require.Equal(t, kv[1], out)
	}

	require.Len(t, cache.buckets, 4)
	acc := 0
	for i := 0; i < 4; i++ {
		clen, err := cache.buckets[i].Len()
		acc += clen
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, clen, len(dataset)/2/4, "at least 50%/nbuckets of all keys in each bucket")
	}
	// because of the limited size of each bucket and the nature of our data,
	// we may have around 10% of our keys evicted in this scenario. removing 1% because we cannot predict
	// accurately what is happening with random data.
	assert.GreaterOrEqual(t, acc, len(dataset)-(len(dataset)*1.0/100.0))
}

func TestLRUStriped_Size(t *testing.T) {
	scache, err := NewLRUStriped(LRUOptions{StripedBuckets: 2, Size: 128})
	require.NoError(t, err)
	cache := scache.(LRUStriped)
	acc := 0
	for _, bucket := range cache.buckets {
		acc += bucket.size
	}
	assert.Equal(t, 128+13+1, acc) // +10% +modulo padding
}

func TestLRUStriped_HashKey(t *testing.T) {
	scache, err := NewLRUStriped(LRUOptions{StripedBuckets: 2, Size: 128})
	require.NoError(t, err)
	cache := scache.(LRUStriped)
	first := cache.hashkeyMapHash("key")
	cache.hashkeyMapHash("other_key_to_ensure_that_result_it’s_not_dependent_on_previous_input")
	second := cache.hashkeyMapHash("key")
	require.Equal(t, first, second)
}

func TestLRUStriped_Get(t *testing.T) {
	cache, err := NewLRUStriped(LRUOptions{StripedBuckets: 4, Size: 128})
	require.NoError(t, err)
	var out string
	require.Equal(t, ErrKeyNotFound, cache.Get("key", &out))
	require.Zero(t, out)

	require.NoError(t, cache.Set("key", "value"))
	require.NoError(t, cache.Get("key", &out))
	require.Equal(t, "value", out)
}

var hashSink uint64

func BenchmarkSum64(b *testing.B) {
	cases := []string{
		"1",
		"22",
		"333",
		model.NewID(),
		model.NewID() + model.NewID(),
	}

	for _, case_ := range cases {
		b.Run(fmt.Sprintf("maphash_string_len_%d", len(case_)), func(b *testing.B) {
			seed := maphash.MakeSeed()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				var h maphash.Hash
				h.SetSeed(seed)
				h.WriteString(case_) // documentation and code says it never fails
				hashSink = h.Sum64()
			}
		})
		b.Run(fmt.Sprintf("xxhash_string_len_%d", len(case_)), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				hashSink = xxhash.Sum64String(case_)
			}
		})
	}
}
