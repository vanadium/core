// Copyright 2022 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build debug
// +build debug

package debug_test

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"v.io/x/ref/runtime/internal/flow/conn/debug"
)

func TestTracking(t *testing.T) {
	cl := debug.CallerCircularList{}

	filter := func(trace string) (out []int) {
		lines := strings.Split(trace, "\n")
		for _, line := range lines {
			if strings.HasSuffix(line, "17:v.io/x/ref/runtime/internal/flow/conn.debug_test.TestTracking") {
				continue
			}
			sidx := strings.Index(line, ":")
			eidx := strings.Index(line, ":v.io/x/ref/runtime/internal/flow/conn/debug_test.TestTracking")
			if sidx > 0 && eidx > 0 {
				l, _ := strconv.Atoi(line[sidx+1 : eidx])
				out = append(out, l)
			}

		}
		return
	}

	cl.Append("a")
	cl.Append("b")

	if got, want := filter(cl.String()), []int{41, 40}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	for i := 0; i < 98; i++ {
		cl.Append(fmt.Sprintf("%v\n", i))
	}

	lines := make([]int, 100)
	for i := 0; i < len(lines); i++ {
		lines[i] = 48
	}
	lines[99] = 40
	lines[98] = 41
	if got, want := filter(cl.String()), lines; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	cl.Append("k")
	for i := 0; i < len(lines); i++ {
		lines[i] = 48
	}
	lines[0] = 61
	lines[99] = 41
	if got, want := filter(cl.String()), lines; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

}
