// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

// Commands to get/set Permissions.

import (
	"fmt"

	"v.io/v23/context"
	"v.io/v23/security"
	"v.io/v23/security/access"
	"v.io/v23/services/device"
	"v.io/v23/verror"
	"v.io/x/lib/cmdline"
	"v.io/x/ref/lib/v23cmd"
)

var cmdGet = &cmdline.Command{
	Runner:   v23cmd.RunnerFunc(runGet),
	Name:     "get",
	Short:    "Get Permissions for the given target.",
	Long:     "Get Permissions for the given target.",
	ArgsName: "<device manager name>",
	ArgsLong: `
<device manager name> can be a Vanadium name for a device manager,
application installation or instance.`,
}

func runGet(ctx *context.T, env *cmdline.Env, args []string) error {
	if expected, got := 1, len(args); expected != got {
		return env.UsageErrorf("get: incorrect number of arguments, expected %d, got %d", expected, got)
	}

	vanaName := args[0]
	objPerms, _, err := device.ApplicationClient(vanaName).GetPermissions(ctx)
	if err != nil {
		return fmt.Errorf("GetPermissions on %s failed: %v", vanaName, err)
	}
	// Convert objPerms (Permissions) into permsEntries for pretty printing.
	entries := make(permsEntries)
	for tag, perms := range objPerms {
		for _, p := range perms.In {
			entries.Tags(string(p))[tag] = false
		}
		for _, b := range perms.NotIn {
			entries.Tags(b)[tag] = true
		}
	}
	fmt.Fprintln(env.Stdout, entries)
	return nil
}

// TODO(caprita): Add unit test logic for 'force set'.
var forceSet bool

var cmdSet = &cmdline.Command{
	Runner:   v23cmd.RunnerFunc(runSet),
	Name:     "set",
	Short:    "Set Permissions for the given target.",
	Long:     "Set Permissions for the given target",
	ArgsName: "<device manager name>  (<blessing> [!]<tag>(,[!]<tag>)*",
	ArgsLong: `
<device manager name> can be a Vanadium name for a device manager,
application installation or instance.

<blessing> is a blessing pattern.
If the same pattern is repeated multiple times in the command, then
the only the last occurrence will be honored.

<tag> is a subset of defined access types ("Admin", "Read", "Write" etc.).
If the access right is prefixed with a '!' then <blessing> is added to the
NotIn list for that right. Using "^" as a "tag" causes all occurrences of
<blessing> in the current AccessList to be cleared.

Examples:
set root/self ^
will remove "root/self" from the In and NotIn lists for all access rights.

set root/self Read,!Write
will add "root/self" to the In list for Read access and the NotIn list
for Write access (and remove "root/self" from both the In and NotIn
lists of all other access rights)`,
}

func init() {
	cmdSet.Flags.BoolVar(&forceSet, "f", false, "Instead of making the AccessLists additive, do a complete replacement based on the specified settings.")
}

func runSet(ctx *context.T, env *cmdline.Env, args []string) error {
	if got := len(args); !((got%2) == 1 && got >= 3) {
		return env.UsageErrorf("set: incorrect number of arguments %d, must be 1 + 2n", got)
	}

	vanaName := args[0]
	pairs := args[1:]

	entries := make(permsEntries)
	for i := 0; i < len(pairs); i += 2 {
		blessing := pairs[i]
		tags, err := parseAccessTags(pairs[i+1])
		if err != nil {
			return env.UsageErrorf("failed to parse access tags for %q: %v", blessing, err)
		}
		entries[blessing] = tags
	}

	// Set the Permissions on the specified names.
	for {
		objPerms, version := make(access.Permissions), ""
		if !forceSet {
			var err error
			if objPerms, version, err = device.ApplicationClient(vanaName).GetPermissions(ctx); err != nil {
				return fmt.Errorf("GetPermissions(%s) failed: %v", vanaName, err)
			}
		}
		for blessingOrPattern, tags := range entries {
			objPerms.Clear(blessingOrPattern) // Clear out any existing references
			for tag, blacklist := range tags {
				if blacklist {
					objPerms.Blacklist(blessingOrPattern, tag)
				} else {
					objPerms.Add(security.BlessingPattern(blessingOrPattern), tag)
				}
			}
		}
		switch err := device.ApplicationClient(vanaName).SetPermissions(ctx, objPerms, version); {
		case err != nil && verror.ErrorID(err) != verror.ErrBadVersion.ID:
			return fmt.Errorf("SetPermissions(%s) failed: %v", vanaName, err)
		case err == nil:
			return nil
		}
		fmt.Fprintln(env.Stderr, "WARNING: trying again because of asynchronous change")
	}
}

var cmdACL = &cmdline.Command{
	Name:  "acl",
	Short: "Tool for setting device manager Permissions",
	Long: `
The acl tool manages Permissions on the device manger, installations and instances.
`,
	Children: []*cmdline.Command{cmdGet, cmdSet},
}
