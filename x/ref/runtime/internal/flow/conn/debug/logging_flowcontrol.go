// Copyright 2022 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build debug
// +build debug

package debug

import (
	"fmt"
	"os"
	"strings"

	"v.io/v23/flow/message"
)

func FlowControl(format string, args ...interface{}) {
	if !debugFlowControl {
		return
	}
	fmt.Fprintf(os.Stderr, format, args...)
}

func FormatCounters(counters []message.Counter) string {
	out := strings.Builder{}
	out.WriteString(fmt.Sprintf("#%v counters:: ", len(counters)))
	i := 0
	max := 40
	for _, v := range counters {
		out.WriteString(fmt.Sprintf("%v:%v ", v.FlowID, v.Tokens))
		if i >= max {
			break
		}
		i++
	}
	return out.String()
}
