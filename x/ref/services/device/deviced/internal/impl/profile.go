// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package impl

import (
	"bytes"
	"errors"
	"os/exec"
	"runtime"
	"strings"

	"v.io/v23/services/build"
	"v.io/v23/services/device"
	"v.io/x/ref/services/internal/profiles"
	"v.io/x/ref/services/profile"
)

// ComputeDeviceProfile generates a description of the runtime
// environment (supported file format, OS, architecture, libraries) of
// the host device.
//
// TODO(jsimsa): Avoid computing the host device description from
// scratch if a recent cached copy exists.
func ComputeDeviceProfile() (*profile.Specification, error) {
	result := profile.Specification{}

	// Find out what the supported operating system, file format, and
	// architecture is.
	var os build.OperatingSystem
	if err := os.SetFromGoOS(runtime.GOOS); err != nil {
		return nil, err
	}
	result.Os = os
	switch os {
	case build.OperatingSystemDarwin:
		result.Format = build.FormatMach
	case build.OperatingSystemLinux:
		result.Format = build.FormatElf
	case build.OperatingSystemWindows:
		result.Format = build.FormatPe
	case build.OperatingSystemAndroid:
		result.Format = build.FormatElf
	default:
		return nil, errors.New("Unsupported operating system: " + os.String())
	}
	var arch build.Architecture
	if err := arch.SetFromGoArch(runtime.GOARCH); err != nil {
		return nil, err
	}
	result.Arch = arch

	// Find out what the installed dynamically linked libraries are.
	switch runtime.GOOS {
	case "linux":
		// For Linux, we identify what dynamically linked libraries are
		// installed by parsing the output of "ldconfig -p".
		command := exec.Command("/sbin/ldconfig", "-p")
		output, err := command.CombinedOutput()
		if err != nil {
			return nil, err
		}
		buf := bytes.NewBuffer(output)
		// Throw away the first line of output from ldconfig.
		if _, err := buf.ReadString('\n'); err != nil {
			return nil, errors.New("Could not identify libraries.")
		}
		// Extract the library name and version from every subsequent line.
		result.Libraries = make(map[profile.Library]struct{})
		line, err := buf.ReadString('\n')
		for err == nil {
			words := strings.Split(strings.Trim(line, " \t\n"), " ")
			if len(words) > 0 {
				tokens := strings.Split(words[0], ".so")
				if len(tokens) != 2 {
					return nil, errors.New("Could not identify library: " + words[0])
				}
				name := strings.TrimPrefix(tokens[0], "lib")
				major, minor := "", ""
				tokens = strings.SplitN(tokens[1], ".", 3)
				if len(tokens) >= 2 {
					major = tokens[1]
				}
				if len(tokens) >= 3 {
					minor = tokens[2]
				}
				result.Libraries[profile.Library{Name: name, MajorVersion: major, MinorVersion: minor}] = struct{}{}
			}
			line, err = buf.ReadString('\n')
		}
	case "darwin":
		// TODO(jsimsa): Implement.
	case "windows":
		// TODO(jsimsa): Implement.
	case "android":
		// TODO(caprita): Implement.
	default:
		return nil, errors.New("Unsupported operating system: " + runtime.GOOS)
	}
	return &result, nil
}

// getProfile gets a profile description for the given profile.
//
// TODO(jsimsa): Avoid retrieving the list of known profiles from a
// remote server if a recent cached copy exists.
func getProfile(name string) (*profile.Specification, error) {
	profiles, err := profiles.GetKnownProfiles()
	if err != nil {
		return nil, err
	}
	for _, p := range profiles {
		if p.Label == name {
			return p, nil
		}
	}
	return nil, nil

	// TODO(jsimsa): This function assumes the existence of a profile
	// server from which the profiles can be retrieved. The profile
	// server is a work in progress. When it exists, the commented out
	// code below should work.
	/*
		var profile profile.Specification
				client, err := r.NewClient()
				if err != nil {
					return nil, verror.New(ErrOperationFailed, nil, fmt.Sprintf("NewClient() failed: %v", err))
				}
				defer client.Close()
			  server := // TODO
				method := "Specification"
				inputs := make([]interface{}, 0)
				call, err := client.StartCall(server + "/" + name, method, inputs)
				if err != nil {
					return nil, verror.New(ErrOperationFailed, nil, fmt.Sprintf("StartCall(%s, %q, %v) failed: %v\n", server + "/" + name, method, inputs, err))
				}
				if err := call.Finish(&profiles); err != nil {
					return nil, verror.New(ErrOperationFailed, nil, fmt.Sprintf("Finish(%v) failed: %v\n", &profiles, err))
				}
		return &profile, nil
	*/
}

// matchProfiles inputs a profile that describes the host device and a
// set of publicly known profiles and outputs a device description that
// identifies the publicly known profiles supported by the host device.
func matchProfiles(p *profile.Specification, known []*profile.Specification) device.Description {
	result := device.Description{Profiles: make(map[string]struct{})}
loop:
	for _, profile := range known {
		if profile.Format != p.Format {
			continue
		}
		if profile.Os != p.Os {
			continue
		}
		if profile.Arch != p.Arch {
			continue
		}
		for library := range profile.Libraries {
			// Current implementation requires exact library name and version match.
			if _, found := p.Libraries[library]; !found {
				continue loop
			}
		}
		result.Profiles[profile.Label] = struct{}{}
	}
	return result
}

// Describe returns a Description containing the profile that matches the
// current device.  It's declared as a variable so we can override it for
// testing.
var Describe = func() (device.Description, error) {
	empty := device.Description{}
	deviceProfile, err := ComputeDeviceProfile()
	if err != nil {
		return empty, err
	}
	knownProfiles, err := profiles.GetKnownProfiles()
	if err != nil {
		return empty, err
	}
	result := matchProfiles(deviceProfile, knownProfiles)
	if len(result.Profiles) == 0 {
		// For now, return "unknown" as the profile, if no known profile
		// matches the device's profile.
		//
		// TODO(caprita): Get rid of this crutch once we have profiles
		// defined for our supported systems; for now it helps us make
		// the integration test work on e.g. Mac.
		result.Profiles["unknown"] = struct{}{}
	}
	return result, nil
}
