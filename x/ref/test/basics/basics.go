// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package basics implements some benchmarks for important basic operations in go.
package basics

// #include "roundtrip.h"
import "C"

import (
	"fmt"
	"testing"
)

func cRoundTrip(b *testing.B) error {
	var test C.struct_rt_test
	if C.rt_init(&test) != 0 {
		return fmt.Errorf("rtInit failure")
	}
	defer C.rt_stop(test)
	ch := make(chan error)
	b.ResetTimer()
	go func() {
		if C.rt_recvn(test, C.int(b.N)) != 0 {
			ch <- fmt.Errorf("rtRecvN failure")
		} else {
			ch <- nil
		}
	}()
	if C.rt_sendn(test, C.int(b.N)) != 0 {
		return fmt.Errorf("rtSendN failure")
	}
	b.StopTimer()
	return <-ch
}
