// Copyright 2022 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !debug

package debug

type WriteqStats struct {
}

func (wqs *WriteqStats) String() string {
	return ""
}

func (wqs *WriteqStats) Next(w *WriterStats) {}

func (wqs *WriteqStats) WaitCall() {}

func (wqs *WriteqStats) WaitReturn() {}

func (wqs *WriteqStats) DoneCall() {}

func (wqs *WriteqStats) DoneReturn() {}

func (wqs *WriteqStats) Receive() {}

func (wqs *WriteqStats) Send() {}

func (wqs *WriteqStats) Bypassed() {}
func (wqs *WriteqStats) Add()      {}

func (wqs *WriteqStats) Rm() {}

type WriterStats struct {
}

func (ws *WriterStats) SetComment(comment string) {}

func (ws *WriterStats) String() string {
	return ""
}

func Writeq(format string, args ...interface{}) {}
