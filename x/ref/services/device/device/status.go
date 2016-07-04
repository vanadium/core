// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"io"

	"v.io/v23/context"
	"v.io/v23/services/device"
	"v.io/x/lib/cmdline"
)

var cmdStatus = &cmdline.Command{
	Name:     "status",
	Short:    "Get device manager or application status.",
	Long:     "Get the status of the device manager or application instances and installations.",
	ArgsName: "<name patterns...>",
	ArgsLong: `
<name patterns...> are vanadium object names or glob name patterns corresponding to the device manager service, or to application installations and instances.`,
}

func init() {
	globify(cmdStatus, runStatus, new(GlobSettings))
}

func runStatus(entry GlobResult, _ *context.T, stdout, _ io.Writer) error {
	switch s := entry.Status.(type) {
	case device.StatusInstance:
		fmt.Fprintf(stdout, "Instance %v [State:%v,Version:%v]\n", entry.Name, s.Value.State, s.Value.Version)
	case device.StatusInstallation:
		fmt.Fprintf(stdout, "Installation %v [State:%v,Version:%v]\n", entry.Name, s.Value.State, s.Value.Version)
	case device.StatusDevice:
		fmt.Fprintf(stdout, "Device Service %v [State:%v,Version:%v]\n", entry.Name, s.Value.State, s.Value.Version)
	default:
		return fmt.Errorf("Status returned unknown type: %T", s)
	}
	return nil
}
