// Copyright 2022 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build debug

package debug

import (
	"fmt"
	"os"
	"strings"
	"sync/atomic"
)

type WriteqStats struct {
	NextID uint64

	MaxSize, Size          uint64
	WaitEntry, WaitExit    uint64
	DoneEntry, DoneExit    uint64
	SignalWait, SignalSend uint64
	Bypass                 uint64
}

func (wqs *WriteqStats) String() string {
	out := strings.Builder{}
	out.WriteString(fmt.Sprintf("---- writeq stats ---\nnextID: %4v\n", atomic.LoadUint64(&wqs.NextID)))
	out.WriteString(fmt.Sprintf("wait: in:out %4v:%4v\n", atomic.LoadUint64(&wqs.WaitEntry), atomic.LoadUint64(&wqs.WaitExit)))
	out.WriteString(fmt.Sprintf("done: in:out %4v:%4v\n", atomic.LoadUint64(&wqs.DoneEntry), atomic.LoadUint64(&wqs.DoneExit)))
	out.WriteString(fmt.Sprintf("signals: wait:sent:bypass %4v:%4v:%4v\n", atomic.LoadUint64(&wqs.SignalWait), atomic.LoadUint64(&wqs.SignalSend), atomic.LoadUint64(&wqs.Bypass)))
	out.WriteString(fmt.Sprintf("size: max:cur: %4v:%4v\n", atomic.LoadUint64(&wqs.MaxSize), atomic.LoadUint64(&wqs.Size)))
	return out.String()
}

func (wqs *WriteqStats) Next(w *WriterStats) {
	w.ID = atomic.AddUint64(&wqs.NextID, 1)
}

func (wqs *WriteqStats) WaitCall() {
	atomic.AddUint64(&wqs.WaitEntry, 1)
}

func (wqs *WriteqStats) WaitReturn() {
	atomic.AddUint64(&wqs.WaitExit, 1)
}

func (wqs *WriteqStats) DoneCall() {
	atomic.AddUint64(&wqs.DoneEntry, 1)
}

func (wqs *WriteqStats) DoneReturn() {
	atomic.AddUint64(&wqs.DoneExit, 1)
}

func (wqs *WriteqStats) Receive() {
	atomic.AddUint64(&wqs.SignalWait, 1)
}

func (wqs *WriteqStats) Send() {
	atomic.AddUint64(&wqs.SignalSend, 1)
}

func (wqs *WriteqStats) Bypassed() {
	atomic.AddUint64(&wqs.Bypass, 1)
}

func (wqs *WriteqStats) Add() {
	// assume this called with a lock held
	wqs.Size++
	if wqs.Size > wqs.MaxSize {
		wqs.MaxSize = wqs.Size
	}
}

func (wqs *WriteqStats) Rm() {
	// assume this called with a lock held
	wqs.Size--
}

type WriterStats struct {
	ID      uint64
	Comment string
}

func (ws *WriterStats) SetComment(comment string) {
	ws.Comment = comment
}

func (ws *WriterStats) String() string {
	return fmt.Sprintf("wid:%4v comment:%s", ws.ID, ws.Comment)
}

func Writeq(format string, args ...interface{}) {
	if !debugWriteq {
		return
	}
	fmt.Fprintf(os.Stderr, format, args...)
}
