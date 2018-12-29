// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package profiles

import (
	//	"bytes"
	//	"errors"
	//	"os/exec"
	//	"runtime"
	//	"strings"

	"v.io/v23/services/build"
	//	"v.io/v23/services/device"
	"v.io/x/ref/services/profile"
)

// GetKnownProfiles gets a list of description for all publicly known
// profiles.
//
// TODO(jsimsa): Avoid retrieving the list of known profiles from a
// remote server if a recent cached copy exists.
func GetKnownProfiles() ([]*profile.Specification, error) {
	return []*profile.Specification{
		{
			Label:       "linux-amd64",
			Description: "",
			Arch:        build.ArchitectureAmd64,
			Os:          build.OperatingSystemLinux,
			Format:      build.FormatElf,
		},
		{
			// Note that linux-386 is used instead of linux-x86 for the
			// label to facilitate generation of a matching label string
			// using the runtime.GOARCH value. In VDL, the 386 architecture
			// is represented using the value X86 because the VDL grammar
			// does not allow identifiers starting with a number.
			Label:       "linux-386",
			Description: "",
			Arch:        build.ArchitectureX86,
			Os:          build.OperatingSystemLinux,
			Format:      build.FormatElf,
		},
		{
			Label:       "linux-arm",
			Description: "",
			Arch:        build.ArchitectureArm,
			Os:          build.OperatingSystemLinux,
			Format:      build.FormatElf,
		},
		{
			Label:       "darwin-amd64",
			Description: "",
			Arch:        build.ArchitectureAmd64,
			Os:          build.OperatingSystemDarwin,
			Format:      build.FormatMach,
		},
		{
			Label:       "android-arm",
			Description: "",
			Arch:        build.ArchitectureArm,
			Os:          build.OperatingSystemAndroid,
			Format:      build.FormatElf,
		},
	}, nil

	// TODO(jsimsa): This function assumes the existence of a profile
	// server from which a list of known profiles can be retrieved. The
	// profile server is a work in progress. When it exists, the
	// commented out code below should work.

	/*
		knownProfiles := make([]profile.Specification, 0)
				client, err := r.NewClient()
				if err != nil {
					return nil,  verror.New(ErrOperationFailed, nil, fmt.Sprintf("NewClient() failed: %v\n", err))
				}
				defer client.Close()
			  server := // TODO
				method := "List"
				inputs := make([]interface{}, 0)
				call, err := client.StartCall(server, method, inputs)
				if err != nil {
					return nil, verror.New(ErrOperationFailed, nil, fmt.Sprintf("StartCall(%s, %q, %v) failed: %v\n", server, method, inputs, err))
				}
				if err := call.Finish(&knownProfiles); err != nil {
					return nil, verror.New(ErrOperationFailed, nil, fmt.Sprintf("Finish(&knownProfile) failed: %v\n", err))
				}
		return knownProfiles, nil
	*/
}
