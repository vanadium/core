// Copyright 2022 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build debug
// +build debug

package debug

import (
	"fmt"
	"os"
	"reflect"
	"strings"

	"v.io/v23/flow/message"
)

func FormatMessage(m message.Message) string {
	out := strings.Builder{}
	out.WriteString(reflect.TypeOf(m).String())
	switch msg := m.(type) {
	case message.Data:
		out.WriteString(fmt.Sprintf(": flow: %3v", msg.ID))
		if len(msg.Payload) > 0 {
			out.WriteString(fmt.Sprintf(": flags: %02x, #%v bytes", msg.Flags, len(msg.Payload[0])))
		} else {
			out.WriteString(fmt.Sprintf(": flags: %02x - no payload", msg.Flags))
		}
	case message.Release:
		out.WriteString(FormatCounters(msg.Counters))
	case message.OpenFlow:
		out.WriteString(fmt.Sprintf(": flow: %3v", msg.ID))
		if len(msg.Payload) > 0 {
			out.WriteString(fmt.Sprintf(": flags: %02x, #%v bytes", msg.Flags, len(msg.Payload[0])))
		} else {
			out.WriteString(fmt.Sprintf(": flags: %02x - no payload", msg.Flags))
		}
	}
	return out.String()
}

func MessagePipe(format string, args ...interface{}) {
	if !debugMessagePipe {
		return
	}
	fmt.Fprintf(os.Stderr, format, args...)
}
