// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"sync"

	"v.io/v23/context"
	"v.io/v23/rpc"
	"v.io/x/ref/runtime/internal/rpc/stress"
)

type impl struct {
	statsMu sync.Mutex
	stats   stress.SumStats // GUARDED_BY(statsMu)

	stop chan struct{}
}

func NewService() (stress.StressServerStub, <-chan struct{}) {
	s := &impl{stop: make(chan struct{})}
	return stress.StressServer(s), s.stop
}

func (s *impl) Echo(_ *context.T, _ rpc.ServerCall, payload []byte) ([]byte, error) {
	return payload, nil
}

func (s *impl) Sum(_ *context.T, _ rpc.ServerCall, arg stress.SumArg) ([]byte, error) {
	sum, err := doSum(&arg)
	if err != nil {
		return nil, err
	}
	s.addSumStats(false, uint64(lenSumArg(&arg)), uint64(len(sum)))
	return sum, nil
}

func (s *impl) SumStream(_ *context.T, call stress.StressSumStreamServerCall) error {
	rStream := call.RecvStream()
	sStream := call.SendStream()
	var bytesRecv, bytesSent uint64
	for rStream.Advance() {
		arg := rStream.Value()
		sum, err := doSum(&arg)
		if err != nil {
			return err
		}
		sStream.Send(sum) //nolint:errcheck
		bytesRecv += uint64(lenSumArg(&arg))
		bytesSent += uint64(len(sum))
	}
	if err := rStream.Err(); err != nil {
		return err
	}
	s.addSumStats(true, bytesRecv, bytesSent)
	return nil
}

func (s *impl) addSumStats(stream bool, bytesRecv, bytesSent uint64) {
	s.statsMu.Lock()
	if stream {
		s.stats.SumStreamCount++
	} else {
		s.stats.SumCount++
	}
	s.stats.BytesRecv += bytesRecv
	s.stats.BytesSent += bytesSent
	s.statsMu.Unlock()
}

func (s *impl) GetSumStats(*context.T, rpc.ServerCall) (stress.SumStats, error) {
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	return s.stats, nil
}

func (s *impl) Stop(*context.T, rpc.ServerCall) error {
	s.stop <- struct{}{}
	return nil
}
