// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"math/rand"
	"sync"

	"v.io/v23/context"
	"v.io/v23/rpc"
	"v.io/x/ref/examples/fortune"
)

// Fortune implements fortune.FortuneServerMethods.
type impl struct {
	fortunes []string
	random   *rand.Rand
	mutex    sync.RWMutex
}

// newImpl returns a Fortune implementation that can be used to create a service.
func NewImpl() fortune.FortuneServerMethods {
	return &impl{
		fortunes: []string{
			"You will reach the heights of success.",
			"Conquer your fears or they will conquer you.",
			"Today is your lucky day!",
		},
		random: rand.New(rand.NewSource(99)),
	}
}

// Get retrieves a random fortune from the fortunes array.
func (f *impl) Get(_ *context.T, _ rpc.ServerCall) (fortune string, err error) {
	f.mutex.RLock()
	defer f.mutex.RUnlock()

	length := len(f.fortunes)
	index := f.random.Intn(length)

	return f.fortunes[index], nil
}

// Add inserts a new fortune into the fortunes array.
func (f *impl) Add(_ *context.T, _ rpc.ServerCall, fortune string) error {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	f.fortunes = append(f.fortunes, fortune)

	return nil
}

// Has returns a boolean that states wether a fortune exists in the fortunes array.
func (f *impl) Has(_ *context.T, _ rpc.ServerCall, fortune string) (bool, error) {
	f.mutex.RLock()
	defer f.mutex.RUnlock()

	exists := contains(f.fortunes, fortune)

	return exists, nil
}

func contains(fortunes []string, fortune string) bool {
	for _, item := range fortunes {
		if item == fortune {
			return true
		}
	}

	return false
}
