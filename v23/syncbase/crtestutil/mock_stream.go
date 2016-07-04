// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package crtestutil

import (
	"sync"

	wire "v.io/v23/services/syncbase"
)

var _ wire.ConflictManagerStartConflictResolverClientCall = (*CrStreamImpl)(nil)

type State struct {
	// ConflictStream state
	isBlocked    bool
	dataLock     sync.Mutex
	Mu           sync.Mutex
	Val          wire.ConflictInfo
	advanceCount int
	ValIndex     int
	// ResolutionStream state variables
	result []wire.ResolutionInfo
}

func (st *State) GetIsBlocked() bool {
	st.dataLock.Lock()
	defer st.dataLock.Unlock()
	return st.isBlocked
}

func (st *State) SetIsBlocked(isBlocked bool) {
	st.dataLock.Lock()
	defer st.dataLock.Unlock()
	st.isBlocked = isBlocked
}

func (st *State) GetAdvanceCount() int {
	st.dataLock.Lock()
	defer st.dataLock.Unlock()
	return st.advanceCount
}

func (st *State) IncrementAdvanceCount() {
	st.dataLock.Lock()
	defer st.dataLock.Unlock()
	st.advanceCount++
}

func (st *State) GetResult() []wire.ResolutionInfo {
	st.dataLock.Lock()
	defer st.dataLock.Unlock()
	resultCopy := make([]wire.ResolutionInfo, len(st.result))
	copy(resultCopy, st.result)
	return resultCopy
}

type CrStreamImpl struct {
	C ConflictStream
	R ResolutionStream
}

func (s *CrStreamImpl) RecvStream() interface {
	Advance() bool
	Value() wire.ConflictInfo
	Err() error
} {
	return recvStreamImpl{s.C}
}

func (s *CrStreamImpl) SendStream() interface {
	Send(item wire.ResolutionInfo) error
	Close() error
} {
	return sendStreamImpl{s.R}
}

func (s *CrStreamImpl) Finish() error {
	return nil
}

type recvStreamImpl struct {
	c ConflictStream
}

func (rs recvStreamImpl) Advance() bool {
	return rs.c.Advance()
}
func (rs recvStreamImpl) Value() wire.ConflictInfo {
	return rs.c.Value()
}
func (rs recvStreamImpl) Err() error {
	return rs.c.Err()
}

type sendStreamImpl struct {
	r ResolutionStream
}

func (ss sendStreamImpl) Send(item wire.ResolutionInfo) error {
	return ss.r.Send(item)
}
func (c sendStreamImpl) Close() error {
	return nil
}

type ConflictStream interface {
	Advance() bool
	Value() wire.ConflictInfo
	Err() error
}

type ResolutionStream interface {
	Send(item wire.ResolutionInfo) error
}

type ConflictStreamImpl struct {
	St        *State
	AdvanceFn func(*State) bool
}

func (cs *ConflictStreamImpl) Advance() bool {
	return cs.AdvanceFn(cs.St)
}
func (cs *ConflictStreamImpl) Value() wire.ConflictInfo {
	return cs.St.Val
}
func (cs *ConflictStreamImpl) Err() error {
	return &TestError{"Stream broken"}
}

type ResolutionStreamImpl struct {
	St *State
}

func (rs *ResolutionStreamImpl) Send(item wire.ResolutionInfo) error {
	// protect read/write to st.Result
	rs.St.dataLock.Lock()
	defer rs.St.dataLock.Unlock()
	//	if rs.St.result == nil {
	//		rs.St.result = []wire.ResolutionInfo{item}
	//		return nil
	//	}
	rs.St.result = append(rs.St.result, item)
	return nil
}

type TestError struct {
	str string
}

func (e *TestError) Error() string {
	return e.str
}
