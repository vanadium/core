// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testutil

import (
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"sync"
	"time"
)

const (
	SeedEnv = "V23_RNG_SEED"
)

// An instance of Random initialized by the InitRandomGenerator function.
var (
	Rand *Random
	once sync.Once
)

const randPanicMsg = "It looks like the singleton random number generator has not been initialized, please call InitRandGenerator."

// Random is a concurrent-access friendly source of randomness.
type Random struct {
	mu   sync.Mutex
	rand *rand.Rand
}

// RandomInt returns a non-negative pseudo-random int.
func (r *Random) RandomInt() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.rand.Int()
}

// RandomIntn returns a non-negative pseudo-random int in the range [0, n).
func (r *Random) RandomIntn(n int) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.rand.Intn(n)
}

// RandomInt63 returns a non-negative 63-bit pseudo-random integer as an int64.
func (r *Random) RandomInt63() int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.rand.Int63()
}

// RandomBytes generates the given number of random bytes.
func (r *Random) RandomBytes(size int) []byte {
	buffer := make([]byte, size)
	randomMutex.Lock()
	defer randomMutex.Unlock()
	// Generate a 10MB of random bytes since that is a value commonly
	// used in the tests.
	if len(random) == 0 {
		random = generateRandomBytes(r, 10<<20)
	}
	if size > len(random) {
		extra := generateRandomBytes(r, size-len(random))
		random = append(random, extra...)
	}
	start := r.RandomIntn(len(random) - size + 1)
	copy(buffer, random[start:start+size])
	return buffer
}

type loggingFunc func(format string, args ...interface{})

// NewRandGenerator creates a new pseudo-random number generator; the seed may
// be supplied by V23_RNG_SEED to allow for reproducing a previous sequence, and
// is printed using the supplied logging function.
func NewRandGenerator(logger loggingFunc) *Random {
	seed := time.Now().UnixNano()
	seedString := os.Getenv(SeedEnv)
	if seedString != "" {
		var err error
		base, bitSize := 0, 64
		seed, err = strconv.ParseInt(seedString, base, bitSize)
		if err != nil {
			panic(fmt.Sprintf("ParseInt(%v, %v, %v) failed: %v", seedString, base, bitSize, err))
		}
	}
	logger("Seeded pseudo-random number generator with %v", seed)
	return &Random{rand: rand.New(rand.NewSource(seed))}
}

// TODO(caprita): Consider deprecating InitRandGenerator in favor of using
// NewRandGenerator directly.  There are several drawbacks to using the global
// singleton Random object:
//
//   - tests that do not call InitRandGenerator themselves could depend on
//   InitRandGenerator having been called by other tests in the same package and
//   stop working when run standalone with test --run
//
//   - conversely, a test case may call InitRandGenerator without actually
//   needing to; it's hard to figure out if some library called by a test
//   actually uses the Random object or not
//
//   - when several test cases share the same Random object, there is
//   interference in the stream of random numbers generated for each test case
//   if run in parallel
//
// All these issues can be trivially addressed if the Random object is created
// and plumbed through the call stack explicitly.

// InitRandGenerator creates an instance of Random in the public variable Rand
// and prints out the seed use when creating the number number generator using
// the supplied logging function.
func InitRandGenerator(logger loggingFunc) {
	once.Do(func() {
		Rand = NewRandGenerator(logger)
	})
}

var (
	random      []byte
	randomMutex sync.Mutex
)

func generateRandomBytes(rand *Random, size int) []byte {
	buffer := make([]byte, size)
	offset := 0
	for {
		bits := rand.RandomInt63()
		for i := 0; i < 8; i++ {
			buffer[offset] = byte(bits & 0xff)
			size--
			if size == 0 {
				return buffer
			}
			offset++
			bits >>= 8
		}
	}
}

// RandomInt returns a non-negative pseudo-random int using the public variable Rand.
func RandomInt() int {
	if Rand == nil {
		panic(randPanicMsg)
	}
	return Rand.RandomInt()
}

// RandomIntn returns a non-negative pseudo-random int in the range [0, n) using
// the public variable Rand.
func RandomIntn(n int) int {
	if Rand == nil {
		panic(randPanicMsg)
	}
	return Rand.RandomIntn(n)
}

// RandomInt63 returns a non-negative 63-bit pseudo-random integer as an int64
// using the public variable Rand.
func RandomInt63() int64 {
	if Rand == nil {
		panic(randPanicMsg)
	}
	return Rand.RandomInt63()
}

// RandomBytes generates the given number of random bytes using
// the public variable Rand.
func RandomBytes(size int) []byte {
	if Rand == nil {
		panic(randPanicMsg)
	}
	return Rand.RandomBytes(size)
}
