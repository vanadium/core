// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package benchmark

import (
	"fmt"
	"math/rand"
	"testing"

	"v.io/x/ref/services/syncbase/store"
)

// RandomGenerator is a helper for generating random data.
type RandomGenerator struct {
	rand.Rand
	data []byte
	pos  int
}

// NewRandomGenerator returns a new generator of pseudo-random byte sequences
// seeded with the given value. Every N bytes produced by this generator can be
// compressed to (compressionRatio * N) bytes.
func NewRandomGenerator(seed int64, compressionRatio float64) *RandomGenerator {
	gen := &RandomGenerator{
		*rand.New(rand.NewSource(seed)),
		[]byte{},
		0,
	}
	for len(gen.data) < 1000*1000 {
		// We generate compressible byte sequences to test Snappy compression
		// engine used by LevelDB.
		gen.data = append(gen.data, gen.compressibleBytes(100, compressionRatio)...)
	}
	return gen
}

// randomBytes generates n pseudo-random bytes from range [' '..'~'].
func (r *RandomGenerator) randomBytes(n int) (bytes []byte) {
	for i := 0; i < n; i++ {
		bytes = append(bytes, byte(' '+r.Intn(95))) // ' ' .. '~'
	}
	return
}

// compressibleBytes generates a sequence of n pseudo-random bytes that can
// be compressed to ~(compressionRatio * n) bytes.
func (r *RandomGenerator) compressibleBytes(n int, compressionRatio float64) (bytes []byte) {
	raw := int(float64(n) * compressionRatio)
	if raw < 1 {
		raw = 1
	}
	rawData := r.randomBytes(raw)
	// Duplicate the random data until we have filled n bytes.
	for len(bytes) < n {
		bytes = append(bytes, rawData...)
	}
	return bytes[0:n]
}

// generate returns a sequence of n pseudo-random bytes.
func (r *RandomGenerator) generate(n int) []byte {
	if r.pos+n > len(r.data) {
		r.pos = 0
		if n >= len(r.data) {
			panic(fmt.Sprintf("length(%d) is too big", n))
		}
	}
	r.pos += n
	return r.data[r.pos-n : r.pos]
}

// Config is a set of settings required to run a benchmark.
type Config struct {
	Rand *RandomGenerator
	// St is the database to use. Initially it should be empty.
	St       store.Store
	KeyLen   int // size of each key
	ValueLen int // size of each value
}

// WriteSequential writes b.N values in sequential key order.
func WriteSequential(b *testing.B, config *Config) {
	doWrite(b, config, true)
}

// WriteRandom writes b.N values in random key order.
func WriteRandom(b *testing.B, config *Config) {
	doWrite(b, config, false)
}

func doWrite(b *testing.B, config *Config, seq bool) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var k int
		if seq {
			k = i
		} else {
			k = config.Rand.Intn(b.N)
		}
		key := []byte(fmt.Sprintf("%0[2]*[1]d", k, config.KeyLen))
		if err := config.St.Put(key, config.Rand.generate(config.ValueLen)); err != nil {
			b.Fatalf("put error: %v", err)
		}
	}
}

// ReadSequential reads b.N values in sequential key order.
func ReadSequential(b *testing.B, config *Config) {
	WriteSequential(b, config)
	b.ResetTimer()
	s := config.St.Scan([]byte("0"), []byte("z"))
	var key, value []byte
	for i := 0; i < b.N; i++ {
		if !s.Advance() {
			b.Fatalf("can't read next value: %v", s.Err())
		}
		key = s.Key(key)
		value = s.Value(value)
	}
	s.Cancel()
}

// ReadRandom reads b.N values in random key order.
func ReadRandom(b *testing.B, config *Config) {
	WriteSequential(b, config)
	b.ResetTimer()
	var value []byte
	var err error
	for i := 0; i < b.N; i++ {
		key := []byte(fmt.Sprintf("%0[2]*[1]d", config.Rand.Intn(b.N), config.KeyLen))
		if value, err = config.St.Get(key, value); err != nil {
			b.Fatalf("can't read value for key %s: %v", key, err)
		}
	}
}

// Overwrite overwrites b.N values in random key order.
func Overwrite(b *testing.B, config *Config) {
	WriteSequential(b, config)
	b.ResetTimer()
	WriteRandom(b, config)
}
