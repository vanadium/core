// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package util

import (
	"os"
	"strings"
	"time"

	wire "v.io/v23/services/syncbase"
	"v.io/v23/verror"
	"v.io/x/lib/vlog"
	"v.io/x/ref/services/syncbase/store"
	"v.io/x/ref/services/syncbase/store/leveldb"
	"v.io/x/ref/services/syncbase/store/memstore"
)

type OpenOptions struct {
	CreateIfMissing bool
	ErrorIfExists   bool
}

// OpenStore opens the given store.Store. OpenOptions are respected to the
// degree possible for the specified engine.
func OpenStore(engine, path string, opts OpenOptions) (store.Store, error) {
	switch engine {
	case "memstore":
		if !opts.CreateIfMissing {
			return nil, verror.New(verror.ErrInternal, nil, "cannot open memstore")
		}
		// By definition, the memstore does not already exist.
		return memstore.New(), nil
	case "leveldb":
		leveldbOpts := leveldb.OpenOptions{
			CreateIfMissing: opts.CreateIfMissing,
			ErrorIfExists:   opts.ErrorIfExists,
		}
		if opts.CreateIfMissing {
			// Note, os.MkdirAll is a noop if the path already exists. We rely on
			// leveldb to enforce ErrorIfExists.
			if err := os.MkdirAll(path, 0700); err != nil {
				return nil, verror.New(verror.ErrInternal, nil, err)
			}
		}
		st, err := leveldb.Open(path, leveldbOpts)
		if err != nil {
			if strings.Contains(err.Error(), "Corruption") {
				vlog.Errorf("leveldb %s is corrupt.  Moving aside. %v", path, err)
				return nil, handleCorruptLevelDB(path)
			}
		}
		return st, err
	default:
		return nil, verror.New(verror.ErrBadArg, nil, engine)
	}
}

// DestroyStore destroys the specified store. Idempotent.
func DestroyStore(engine, path string) error {
	switch engine {
	case "memstore":
		// memstore doesn't persist any data on the disc, do nothing.
		return nil
	case "leveldb":
		if err := os.RemoveAll(path); err != nil {
			return verror.New(verror.ErrInternal, nil, err)
		}
		return nil
	default:
		return verror.New(verror.ErrBadArg, nil, engine)
	}
}

// Moves a corrupt store aside.  The app will be responsible for creating a new
// one. Returns an error containing the path of the old store in case the user
// wants to try to debug it.
func handleCorruptLevelDB(path string) error {
	newPath := path + ".corrupt." + time.Now().Format(time.RFC3339)
	if err := os.Rename(path, newPath); err != nil {
		return verror.New(verror.ErrInternal, nil, "leveldb corrupt but could not move aside: "+err.Error())
	}
	return wire.NewErrCorruptDatabase(nil, newPath)
}
