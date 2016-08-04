// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testutil

import (
	"strings"
	"sync"

	"v.io/v23/naming"
	"v.io/v23/rpc"
)

type MockServer struct {
	mu sync.Mutex

	eps   []naming.Endpoint // GUARDED_BY(mu)
	dirty chan struct{}     // GUARDED_BY(mu)
}

func (s *MockServer) AddName(string) error    { return nil }
func (s *MockServer) RemoveName(string)       {}
func (s *MockServer) Closed() <-chan struct{} { return nil }
func (s *MockServer) Status() rpc.ServerStatus {
	defer s.mu.Unlock()
	s.mu.Lock()
	return rpc.ServerStatus{
		Endpoints: s.eps,
		Dirty:     s.dirty,
	}
}

func (s *MockServer) UpdateNetwork(eps []naming.Endpoint) {
	defer s.mu.Unlock()
	s.mu.Lock()
	s.eps = eps
	close(s.dirty)
	s.dirty = make(chan struct{})
}

func NewMockServer(eps []naming.Endpoint) *MockServer {
	return &MockServer{
		eps:   eps,
		dirty: make(chan struct{}),
	}
}

func ToEndpoints(addrs ...string) []naming.Endpoint {
	eps := make([]naming.Endpoint, len(addrs))
	for i, addr := range addrs {
		addr = strings.TrimPrefix(addr, "/")
		eps[i], _ = naming.ParseEndpoint(addr)
	}
	return eps
}
