// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package build

// SetFromGoArch assigns the GOARCH string label to x. In particular,
// it takes care of mapping "386" to "x86" as the former is not a
// valid VDL enum value.
func (x *Architecture) SetFromGoArch(label string) error {
	switch label {
	case "386":
		return x.Set("x86")
	default:
		return x.Set(label)
	}
}

// SetFromGoOS assigns the GOOS string label to x.
func (x *OperatingSystem) SetFromGoOS(label string) error {
	return x.Set(label)
}

// ToGoArch returns a GOARCH string label for the given Architecture.
// In particular, it takes care of mapping "x86" to "386" as the latter
// is not a valid VDL enum value.
func (arch Architecture) ToGoArch() string {
	switch arch {
	case ArchitectureAmd64:
		return "amd64"
	case ArchitectureArm:
		return "arm"
	case ArchitectureX86:
		return "386"
	default:
		return "unknown"
	}
}

// ToGoOS returns a GOOS string label for the given Architecture.
func (os OperatingSystem) ToGoOS() string {
	switch os {
	case OperatingSystemDarwin:
		return "darwin"
	case OperatingSystemLinux:
		return "linux"
	case OperatingSystemWindows:
		return "windows"
	case OperatingSystemAndroid:
		return "android"
	default:
		return "unknown"
	}
}
