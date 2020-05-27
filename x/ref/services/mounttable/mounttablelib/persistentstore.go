// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mounttablelib

import (
	"encoding/json"
	"io"
	"os"
	"path"
	"strings"
	"sync"

	"v.io/v23/context"

	"v.io/x/ref/internal/logger"
)

type store struct {
	l   sync.Mutex
	mt  *mountTable
	dir string
	enc *json.Encoder
	f   *os.File
}

type storeElement struct {
	N string // Name of affected node
	V VersionedPermissions
	D bool   // True if the subtree at N has been deleted
	C string // Creator
}

// newPersistentStore will read the permissions log from the directory and apply them to the
// in memory tree.  It will then write a new file from the in memory tree and any new permission
// changes will be appened to this file.  By writing into a new file, we effectively compress
// the permissions file since any set permissions that have been deleted or overwritten will be
// lost.
//
// The code manages three files in the directory 'dir':
//   persistent.permslog - the log of permissions.  A new log entry is added with each SetPermissions or
//      Delete RPC.
//   tmp.permslog - a temporary file created whenever we restart.  Once we write the current state into it,
//      it will be renamed persistent.perms becoming the new log.
//   old.permslog - the previous version of persistent.perms.  This is left around primarily for debugging
//      and as an emergency backup.
func newPersistentStore(ctx *context.T, mt *mountTable, dir string) persistence { //nolint:gocyclo
	s := &store{mt: mt, dir: dir}
	file := path.Join(dir, "persistent.permslog")
	tmp := path.Join(dir, "tmp.permslog")
	old := path.Join(dir, "old.permslog")

	// If the permissions file doesn't exist, try renaming the temporary one.
	f, err := os.Open(file)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Global().Fatalf("cannot open %s: %s", file, err)
		}
		if _, err := os.Stat(tmp); err == nil {
			if err := os.Rename(tmp, file); err != nil {
				logger.Global().Fatalf("cannot rename %s to %s: %s", tmp, file, err)
			}
		}
		if f, err = os.Open(file); err != nil && !os.IsNotExist(err) {
			logger.Global().Fatalf("cannot open %s: %s", file, err)
		}
	} else {
		os.Remove(tmp)
	}

	// Parse the permissions file and apply it to the in memory tree.
	if f != nil {
		if err := s.parseLogFile(ctx, f); err != nil {
			f.Close()
			// Log the error but keep going.  There's not much else we can do.
			logger.Global().Infof("parsing old persistent permissions file %s: %s", file, err)
		}
		f.Close()
	}

	// Write the permissions to a temporary file.  This compresses
	// the file since it writes out only the end state.
	f, err = os.OpenFile(tmp, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0600)
	if err != nil {
		// Log the error but keep going, don't compress, just append to the current file.
		logger.Global().Infof("can't rewrite persistent permissions file %s: %s", file, err)
		if f, err = os.OpenFile(file, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0600); err != nil {
			logger.Global().Fatalf("can't append to log %s: %s", file, err)
		}
		if _, err := f.Seek(0, 2); err != nil {
			logger.Global().Fatalf("can't seek to end of %s: %s", file, err)
		}
		s.enc = json.NewEncoder(f)
		return s
	}
	s.enc = json.NewEncoder(f)
	if err := s.depthFirstPersist(mt.root, ""); err != nil {
		ctx.Infof("depthFirstPersist of %v: %v", mt.root, err)
	}
	f.Close()

	// Switch names and remove the old file.
	if err := os.Remove(old); err != nil {
		ctx.Infof("removing %s: %s", old, err)
	}
	if err := os.Rename(file, old); err != nil {
		ctx.Infof("renaming %s to %s: %s", file, old, err)
	}
	if err := os.Rename(tmp, file); err != nil {
		ctx.Fatalf("renaming %s to %s: %s", tmp, file, err)
	}

	// Reopen the new log file.  We could have just kept around the encoder used
	// to create it but that assumes that, after the Rename above, the s.f still
	// points to the same file.  Only true on Unix like file systems.
	f, err = os.OpenFile(file, os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		ctx.Fatalf("can't open %s: %s", file, err)
	}
	if _, err := f.Seek(0, 2); err != nil {
		ctx.Fatalf("can't seek to end of %s: %s", file, err)
	}
	s.enc = json.NewEncoder(f)
	return s
}

// parseLogFile reads a file and parses the contained VersionedPermissions .
func (s *store) parseLogFile(ctx *context.T, f *os.File) error {
	if f == nil {
		return nil
	}
	ctx.VI(2).Infof("parseLogFile(%s)", f.Name())
	mt := s.mt
	decoder := json.NewDecoder(f)
	cc := &callContext{ctx: ctx,
		create:       true,
		ignorePerms:  true,
		ignoreLimits: true,
	}
	for {
		var e storeElement
		if err := decoder.Decode(&e); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		elems := strings.Split(e.N, "/")
		cc.creator = e.C
		n, err := mt.findNode(cc, elems, nil, nil)
		if n == nil {
			continue
		}
		if err == nil {
			if e.D {
				mt.deleteNode(n.parent, elems[len(elems)-1])
				ctx.VI(2).Infof("deleted %s", e.N)
			} else {
				n.vPerms = &e.V
				n.explicitPermissions = true
				ctx.VI(2).Infof("added versions permissions %v to %s", e.V, e.N)
			}
		}
		n.parent.Unlock()
		n.Unlock()
	}
	return nil
}

// depthFirstPersist performs a recursive depth first traversal logging any explicit permissions.
// Doing this immediately after reading in a log file effectively compresses the log file since
// any duplicate or deleted entries disappear.
func (s *store) depthFirstPersist(n *node, name string) error {
	if n.explicitPermissions {
		if err := s.persistPerms(name, n.creator, n.vPerms); err != nil {
			return err
		}
	}
	for nodeName, c := range n.children {
		return s.depthFirstPersist(c, path.Join(name, nodeName))
	}
	return nil
}

// persistPerms appends a changed permission to the log.
func (s *store) persistPerms(name, creator string, vPerms *VersionedPermissions) error {
	s.l.Lock()
	defer s.l.Unlock()
	e := storeElement{N: name, V: *vPerms, C: creator}
	return s.enc.Encode(&e)
}

// persistDelete appends a single deletion to the log.
func (s *store) persistDelete(name string) error {
	s.l.Lock()
	defer s.l.Unlock()
	e := storeElement{N: name, D: true}
	return s.enc.Encode(&e)
}

func (s *store) close() {
	s.f.Close()
}
