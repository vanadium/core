// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package pathperms provides a library to assist servers implementing
// GetPermissions/SetPermissions functions and authorizers where there are
// path-specific Permissions stored individually in files.
// TODO(rjkroege): Add unit tests.
package pathperms

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/security"
	"v.io/v23/security/access"
	"v.io/v23/verror"
	"v.io/x/ref/lib/security/serialization"
)

const (
	sigName   = "signature"
	permsName = "data"
)

var (
	errOperationFailed = errors.New("operation failed")
)

type pathEntry struct {
	lk sync.Mutex
	c  int
}

// PathStore manages storage of a set of Permissions in the filesystem where each
// path identifies a specific Permissions in the set. PathStore synchronizes
// access to its member Permissions.
type PathStore struct {
	pthlks    map[string]*pathEntry
	lk        sync.Mutex
	ctx       *context.T
	principal security.Principal
}

// NewPathStore creates a new instance of the lock map that uses
// principal to sign stored Permissions files.
func NewPathStore(ctx *context.T) *PathStore {
	return &PathStore{pthlks: make(map[string]*pathEntry), ctx: ctx, principal: v23.GetPrincipal(ctx)}
}

// Get returns the Permissions from the data file in dir.
func (store *PathStore) Get(dir string) (access.Permissions, string, error) {
	permspath := filepath.Join(dir, permsName)
	sigpath := filepath.Join(dir, sigName)
	defer store.lockPath(dir)()
	return getCore(store.ctx, permspath, sigpath)
}

// TODO(rjkroege): Improve lock handling.
func (store *PathStore) lockPath(dir string) func() {
	store.lk.Lock()
	pe, contains := store.pthlks[dir]
	if !contains {
		pe = &pathEntry{}
		store.pthlks[dir] = pe
	}
	pe.c++
	store.lk.Unlock()
	pe.lk.Lock()

	return func() {
		pe.lk.Unlock()
		store.lk.Lock()
		pe.c--
		if pe.c == 0 {
			delete(store.pthlks, dir)
		}
		store.lk.Unlock()
	}
}

func getCore(ctx *context.T, permspath, sigpath string) (access.Permissions, string, error) {
	principal := v23.GetPrincipal(ctx)
	f, err := os.Open(permspath)
	if err != nil {
		// This path is rarely a fatal error so log informationally only.
		ctx.VI(2).Infof("os.Open(%s) failed: %v", permspath, err)
		return nil, "", err
	}
	defer f.Close()

	s, err := os.Open(sigpath)
	if err != nil {
		ctx.Errorf("Signatures for Permissions are required: %s unavailable: %v", permspath, err)
		return nil, "", errOperationFailed
	}
	defer s.Close()

	// read and verify the signature of the perms file
	vf, err := serialization.NewVerifyingReader(f, s, principal.PublicKey())
	if err != nil {
		ctx.Errorf("NewVerifyingReader() failed: %v (perms=%s, sig=%s)", err, permspath, sigpath)
		return nil, "", errOperationFailed
	}

	perms, err := access.ReadPermissions(vf)
	if err != nil {
		ctx.Errorf("ReadPermissions(%s) failed: %v", permspath, err)
		return nil, "", err
	}
	version, err := ComputeVersion(perms)
	if err != nil {
		ctx.Errorf("pathperms.ComputeVersion failed: %v", err)
		return nil, "", err
	}
	return perms, version, nil
}

// Set writes the specified Permissions to the provided directory with
// enforcement of version synchronization mechanism and locking.
func (store *PathStore) Set(dir string, perms access.Permissions, version string) error {
	_, err := store.SetShareable(dir, perms, version, false, true)
	return err
}

// SetIfAbsent writes the specified Permissions to the provided directory only
// if they don't already exist.  Returns true if the permissions were written,
// and false otherwise (the error is nil if the permissions already exist).
func (store *PathStore) SetIfAbsent(dir string, perms access.Permissions) (bool, error) {
	return store.SetShareable(dir, perms, "", false, false)
}

// SetShareable writes the specified Permissions to the provided
// directory with enforcement of version synchronization mechanism and
// locking with file modes that will give the application read-only
// access to the permissions file.
func (store *PathStore) SetShareable(dir string, perms access.Permissions, version string, shareable, overwrite bool) (bool, error) {
	permspath := filepath.Join(dir, permsName)
	sigpath := filepath.Join(dir, sigName)
	defer store.lockPath(dir)()
	_, oversion, err := getCore(store.ctx, permspath, sigpath)
	if err != nil && !os.IsNotExist(err) {
		return false, errOperationFailed
	}
	if !overwrite && err == nil {
		// If overwrite is not set, we need the perms to not already
		// exist; as per previous if test, os.IsNotExist(err) iff err !=
		// nil.
		return false, nil
	}
	if len(version) > 0 && version != oversion {
		return false, verror.ErrorfBadVersion(nil, "version is out of date")
	}
	if err := write(store.ctx, permspath, sigpath, dir, perms, shareable); err != nil {
		return false, err
	}
	return true, nil
}

// Delete removes the permissions stored in the specified directory.
func (store *PathStore) Delete(dir string) error {
	// NOTE: we're not using RemoveAll(dir) out of an excess of caution (in
	// case a bad dir is passed in).
	permspath := filepath.Join(dir, permsName)
	if err := os.Remove(permspath); err != nil {
		return err
	}
	sigpath := filepath.Join(dir, sigName)
	if err := os.Remove(sigpath); err != nil {
		return err
	}
	if err := os.Remove(dir); err != nil {
		return err
	}
	return nil
}

// write writes the specified Permissions to the permsFile with a
// signature in sigFile.
func write(ctx *context.T, permsFile, sigFile, dir string, perms access.Permissions, shareable bool) error {
	principal := v23.GetPrincipal(ctx)
	filemode := os.FileMode(0600)
	dirmode := os.FileMode(0700)
	if shareable {
		filemode = os.FileMode(0644)
		dirmode = os.FileMode(0711)
	}

	// Create dir directory if it does not exist
	if err := os.MkdirAll(dir, dirmode); err != nil {
		ctx.Errorf("Failed to create directory tree %v data:%v", dir, err)
		return errOperationFailed
	}
	// Save the object to temporary data and signature files, and then move
	// those files to the actual data and signature file.
	data, err := ioutil.TempFile(dir, permsName)
	if err != nil {
		ctx.Errorf("Failed to open tmpfile data:%v", err)
		return errOperationFailed
	}
	defer os.Remove(data.Name())
	sig, err := ioutil.TempFile(dir, sigName)
	if err != nil {
		ctx.Errorf("Failed to open tmpfile sig:%v", err)
		return errOperationFailed
	}
	defer os.Remove(sig.Name())
	writer, err := serialization.NewSigningWriteCloser(data, sig, principal, nil)
	if err != nil {
		ctx.Errorf("Failed to create NewSigningWriteCloser:%v", err)
		return errOperationFailed
	}
	if err = access.WritePermissions(writer, perms); err != nil {
		ctx.Errorf("Failed to SavePermissions:%v", err)
		return errOperationFailed
	}
	if err = writer.Close(); err != nil {
		ctx.Errorf("Failed to Close() SigningWriteCloser:%v", err)
		return errOperationFailed
	}
	if err := os.Rename(data.Name(), permsFile); err != nil {
		ctx.Errorf("os.Rename() failed:%v", err)
		return errOperationFailed
	}
	if err := os.Chmod(permsFile, filemode); err != nil {
		ctx.Errorf("os.Chmod() failed:%v", err)
		return errOperationFailed
	}
	if err := os.Rename(sig.Name(), sigFile); err != nil {
		ctx.Errorf("os.Rename() failed:%v", err)
		return errOperationFailed
	}
	if err := os.Chmod(sigFile, filemode); err != nil {
		ctx.Errorf("os.Chmod() failed:%v", err)
		return errOperationFailed
	}
	return nil
}

func (store *PathStore) PermsForPath(ctx *context.T, path string) (access.Permissions, bool, error) {
	perms, _, err := store.Get(path)
	if os.IsNotExist(err) {
		return nil, true, nil
	} else if err != nil {
		return nil, false, err
	}
	return perms, false, nil
}

// PrefixPatterns creates a pattern containing all of the prefix patterns of the
// provided blessings.
func PrefixPatterns(blessings []string) []security.BlessingPattern {
	var patterns []security.BlessingPattern
	for _, b := range blessings {
		patterns = append(patterns, security.BlessingPattern(b).PrefixPatterns()...)
	}
	return patterns
}

// PermissionsForBlessings creates the Permissions list that should be used with
// a newly created object.
func PermissionsForBlessings(blessings []string) access.Permissions {
	perms := make(access.Permissions)

	// Add the invoker's blessings and all its prefixes.
	for _, p := range PrefixPatterns(blessings) {
		for _, tag := range access.AllTypicalTags() {
			perms.Add(p, string(tag))
		}
	}
	return perms
}

// NilAuthPermissions creates Permissions that mimics the default authorization
// policy (i.e., Permissions is matched by all blessings that are either
// extensions of one of the local blessings or can be extended to form one of
// the local blessings.)
func NilAuthPermissions(ctx *context.T, call security.Call) access.Permissions {
	perms := make(access.Permissions)
	lb := security.LocalBlessingNames(ctx, call)
	for _, p := range PrefixPatterns(lb) {
		for _, tag := range access.AllTypicalTags() {
			perms.Add(p, string(tag))
		}
	}
	return perms
}
