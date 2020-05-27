// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package impl

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"v.io/v23/context"
	"v.io/v23/services/device"
	"v.io/v23/verror"
	"v.io/x/ref/services/device/internal/errors"
)

// This file contains the various routines that the device manager uses
// to tidy up its persisted but no longer necessary state.

const aboutOneDay = time.Hour * 24

func oldEnoughToTidy(fi os.FileInfo, now time.Time) bool {
	return fi.ModTime().Add(aboutOneDay).Before(now)
}

// AutomaticTidyingInterval defaults to 1 day.
// Settable for tests.
var AutomaticTidyingInterval = time.Hour * 24

func shouldDelete(idir, suffix string, now time.Time) (bool, error) {
	fi, err := os.Stat(filepath.Join(idir, suffix))
	if err != nil {
		return false, err
	}

	return oldEnoughToTidy(fi, now), nil
}

// MockableNow is exposed for replacability in tests.
var MockableNow = time.Now

// shouldDeleteInstallation returns true if the tidying policy holds
// for this installation.
func shouldDeleteInstallation(idir string, now time.Time) (bool, error) {
	return shouldDelete(idir, device.InstallationStateUninstalled.String(), now)
}

// shouldDeleteInstance returns true if the tidying policy holds
// that the instance should be deleted.
func shouldDeleteInstance(idir string, now time.Time) (bool, error) {
	return shouldDelete(idir, device.InstanceStateDeleted.String(), now)
}

type pthError struct {
	pth string
	err error
}

func pruneDeletedInstances(ctx *context.T, root string, now time.Time) error {
	paths, err := filepath.Glob(filepath.Join(root, "app*", "installation*", "instances", "instance*"))
	if err != nil {
		return err
	}

	allerrors := make([]pthError, 0)

	for _, pth := range paths {
		state, err := getInstanceState(pth)
		if err != nil {
			allerrors = append(allerrors, pthError{pth, err})
			continue
		}
		if state != device.InstanceStateDeleted {
			continue
		}

		shouldDelete, err := shouldDeleteInstance(pth, now)
		if err != nil {
			allerrors = append(allerrors, pthError{pth, err})
			continue
		}

		if shouldDelete {
			if err := suidHelper.deleteFileTree(ctx, pth, nil, nil); err != nil {
				allerrors = append(allerrors, pthError{pth, err})
			}
		}
	}
	return processErrors(ctx, allerrors)
}

func processErrors(ctx *context.T, allerrors []pthError) error {
	if len(allerrors) > 0 {
		errormessages := make([]string, 0, len(allerrors))
		for _, ep := range allerrors {
			errormessages = append(errormessages, fmt.Sprintf("path: %s failed: %v", ep.pth, ep.err))
		}
		return verror.New(errors.ErrOperationFailed, ctx, "Some older instances could not be deleted: %s", strings.Join(errormessages, ", "))
	}
	return nil
}

func pruneUninstalledInstallations(ctx *context.T, root string, now time.Time) error {
	// Read all the Uninstalled installations into a map.
	installationPaths, err := filepath.Glob(filepath.Join(root, "app*", "installation*"))
	if err != nil {
		return err
	}
	pruneCandidates := make(map[string]struct{}, len(installationPaths))
	for _, p := range installationPaths {
		state, err := getInstallationState(p)
		if err != nil {
			return err
		}

		if state != device.InstallationStateUninstalled {
			continue
		}

		pruneCandidates[p] = struct{}{}
	}

	instancePaths, err := filepath.Glob(filepath.Join(root, "app*", "installation*", "instances", "instance*", "installation"))
	if err != nil {
		return err
	}

	allerrors := make([]pthError, 0)

	// Filter out installations that are still owned by an instance. Note
	// that pruneUninstalledInstallations runs after
	// pruneDeletedInstances so that freshly-pruned Instances will not
	// retain the Installation.
	for _, idir := range instancePaths {
		installPath, err := os.Readlink(idir)
		if err != nil {
			allerrors = append(allerrors, pthError{idir, err})
			continue
		}
		delete(pruneCandidates, installPath)
	}

	// All remaining entries in pruneCandidates are not referenced by
	// any instance.
	for pth := range pruneCandidates {
		shouldDelete, err := shouldDeleteInstallation(pth, now)
		if err != nil {
			allerrors = append(allerrors, pthError{pth, err})
			continue
		}

		if shouldDelete {
			if err := suidHelper.deleteFileTree(ctx, pth, nil, nil); err != nil {
				allerrors = append(allerrors, pthError{pth, err})
			}
		}
	}
	return processErrors(ctx, allerrors)
}

// pruneOldLogs removes logs more than a day old. Symlinks (the
// canonical log file name) the (newest) log files that they point to
// are preserved.
func pruneOldLogs(ctx *context.T, root string, now time.Time) error {
	logPaths, err := filepath.Glob(filepath.Join(root, "app*", "installation*", "instances", "instance*", "logs", "*"))
	if err != nil {
		return err
	}

	pruneCandidates := make(map[string]struct{}, len(logPaths))
	for _, p := range logPaths {
		pruneCandidates[p] = struct{}{}
	}

	allerrors := make([]pthError, 0)
	for p := range pruneCandidates {
		fi, err := os.Stat(p)
		if err != nil {
			allerrors = append(allerrors, pthError{p, err})
			delete(pruneCandidates, p)
			continue
		}

		if fi.Mode()&os.ModeSymlink != 0 {
			delete(pruneCandidates, p)
			target, err := os.Readlink(p)
			if err != nil {
				allerrors = append(allerrors, pthError{p, err})
				continue
			}
			delete(pruneCandidates, target)
			continue
		}

		if !oldEnoughToTidy(fi, now) {
			delete(pruneCandidates, p)
		}
	}

	for pth := range pruneCandidates {
		if err := suidHelper.deleteFileTree(ctx, pth, nil, nil); err != nil {
			allerrors = append(allerrors, pthError{pth, err})
		}
	}
	return processErrors(ctx, allerrors)
}

// tidyHarness runs device manager cleanup operations
func tidyHarness(ctx *context.T, root string) error {
	now := MockableNow()

	if err := pruneDeletedInstances(ctx, root, now); err != nil {
		return err
	}

	if err := pruneUninstalledInstallations(ctx, root, now); err != nil {
		return err
	}

	return pruneOldLogs(ctx, root, now)
}

// tidyDaemon runs in a Go routine, processing requests to tidy
// or tidying on a schedule.
func tidyDaemon(ctx *context.T, c <-chan tidyRequests, root string) {
	for {
		select {
		case req, ok := <-c:
			if !ok {
				return
			}
			req.bc <- tidyHarness(req.ctx, root)
		case <-time.After(AutomaticTidyingInterval):
			if err := tidyHarness(nil, root); err != nil {
				ctx.Errorf("tidyDaemon failed to tidy: %v", err)
			}
		}

	}
}

type tidyRequests struct {
	ctx *context.T
	bc  chan<- error
}

func newTidyingDaemon(ctx *context.T, root string) chan<- tidyRequests {
	c := make(chan tidyRequests)
	go tidyDaemon(ctx, c, root)
	return c
}
