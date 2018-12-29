// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

// TODO(caprita): Rename to update_revert.go

import (
	"fmt"
	"io"
	"time"

	"v.io/v23/context"
	"v.io/v23/services/device"
	"v.io/v23/verror"

	"v.io/x/lib/cmdline"
	"v.io/x/ref/services/device/internal/errors"
)

var cmdUpdate = &cmdline.Command{
	Name:     "update",
	Short:    "Update the device manager or applications.",
	Long:     "Update the device manager or application instances and installations",
	ArgsName: "<name patterns...>",
	ArgsLong: `
<name patterns...> are vanadium object names or glob name patterns corresponding to the device manager service, or to application installations and instances.`,
}

func init() {
	globify(cmdUpdate, runUpdate, &GlobSettings{
		HandlerParallelism:      KindParallelism,
		InstanceStateFilter:     ExcludeInstanceStates(device.InstanceStateDeleted),
		InstallationStateFilter: ExcludeInstallationStates(device.InstallationStateUninstalled),
	})
}

var cmdRevert = &cmdline.Command{
	Name:     "revert",
	Short:    "Revert the device manager or applications.",
	Long:     "Revert the device manager or application instances and installations to a previous version of their current version",
	ArgsName: "<name patterns...>",
	ArgsLong: `
<name patterns...> are vanadium object names or glob name patterns corresponding to the device manager service, or to application installations and instances.`,
}

func init() {
	globify(cmdRevert, runRevert, &GlobSettings{
		HandlerParallelism:      KindParallelism,
		InstanceStateFilter:     ExcludeInstanceStates(device.InstanceStateDeleted),
		InstallationStateFilter: ExcludeInstallationStates(device.InstallationStateUninstalled),
	})
}

func instanceIsRunning(ctx *context.T, von string) (bool, error) {
	status, err := device.ApplicationClient(von).Status(ctx)
	if err != nil {
		return false, fmt.Errorf("Failed to get status for instance %q: %v", von, err)
	}
	s, ok := status.(device.StatusInstance)
	if !ok {
		return false, fmt.Errorf("Status for instance %q of wrong type (%T)", von, status)
	}
	return s.Value.State == device.InstanceStateRunning, nil
}

var revertOrUpdate = map[bool]string{true: "revert", false: "update"}
var revertOrUpdateMethod = map[bool]string{true: "Revert", false: "Update"}
var revertOrUpdateNoOp = map[bool]string{true: "no previous version available", false: "already up to date"}

func changeVersionInstance(ctx *context.T, stdout, stderr io.Writer, name string, status device.StatusInstance, revert bool) (retErr error) {
	if status.Value.State == device.InstanceStateRunning {
		if err := device.ApplicationClient(name).Kill(ctx, killDeadline); err != nil {
			// Check the app's state again in case we killed it,
			// nevermind any errors.  The sleep is because Kill
			// currently (4/29/15) returns asynchronously with the
			// device manager shooting the app down.
			time.Sleep(time.Second)
			running, rerr := instanceIsRunning(ctx, name)
			if rerr != nil {
				return rerr
			}
			if running {
				return fmt.Errorf("Kill failed: %v", err)
			}
			fmt.Fprintf(stderr, "WARNING for \"%s\": recovered from Kill error (%s). Proceeding with %s.\n", name, err, revertOrUpdate[revert])
		}
		// App was running, and we killed it, so we need to run it again
		// after the update/revert.
		defer func() {
			if err := device.ApplicationClient(name).Run(ctx); err != nil {
				err = fmt.Errorf("Run failed: %v", err)
				if retErr == nil {
					retErr = err
				} else {
					fmt.Fprintf(stderr, "ERROR for \"%s\": %v.\n", name, err)
				}
			}
		}()
	}
	// Update/revert the instance.
	var err error
	if revert {
		err = device.ApplicationClient(name).Revert(ctx)
	} else {
		err = device.ApplicationClient(name).Update(ctx)
	}
	switch {
	case err == nil:
		fmt.Fprintf(stdout, "Successful %s of version for instance \"%s\".\n", revertOrUpdate[revert], name)
		return nil
	case verror.ErrorID(err) == errors.ErrUpdateNoOp.ID:
		// TODO(caprita): Ideally, we wouldn't even attempt a kill /
		// restart if the update/revert is a no-op.
		fmt.Fprintf(stdout, "Instance \"%s\": %s.\n", name, revertOrUpdateNoOp[revert])
		return nil
	default:
		return fmt.Errorf("%s failed: %v", revertOrUpdateMethod[revert], err)
	}
}

func changeVersionOne(ctx *context.T, what string, stdout, stderr io.Writer, name string, revert bool) error {
	var err error
	if revert {
		err = device.ApplicationClient(name).Revert(ctx)
	} else {
		err = device.ApplicationClient(name).Update(ctx)
	}
	switch {
	case err == nil:
		fmt.Fprintf(stdout, "Successful %s of version for %s \"%s\".\n", revertOrUpdate[revert], what, name)
		return nil
	case verror.ErrorID(err) == errors.ErrUpdateNoOp.ID:
		fmt.Fprintf(stdout, "%s \"%s\": %s.\n", what, name, revertOrUpdateNoOp[revert])
		return nil
	default:
		return fmt.Errorf("%s failed: %v", revertOrUpdateMethod[revert], err)
	}
}

func changeVersion(entry GlobResult, ctx *context.T, stdout, stderr io.Writer, revert bool) error {
	switch entry.Kind {
	case ApplicationInstanceObject:
		return changeVersionInstance(ctx, stdout, stderr, entry.Name, entry.Status.(device.StatusInstance), revert)
	case ApplicationInstallationObject:
		return changeVersionOne(ctx, "installation", stdout, stderr, entry.Name, revert)
	case DeviceServiceObject:
		return changeVersionOne(ctx, "device service", stdout, stderr, entry.Name, revert)
	default:
		return fmt.Errorf("unhandled object kind %v", entry.Kind)
	}
}

func runUpdate(entry GlobResult, ctx *context.T, stdout, stderr io.Writer) error {
	return changeVersion(entry, ctx, stdout, stderr, false)
}

func runRevert(entry GlobResult, ctx *context.T, stdout, stderr io.Writer) error {
	return changeVersion(entry, ctx, stdout, stderr, true)
}
