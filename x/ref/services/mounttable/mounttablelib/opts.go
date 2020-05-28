// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mounttablelib

import (
	"flag"
)

type Opts struct {
	MountName  string
	AclFile    string //nolint:golint // API change required.
	NhName     string
	PersistDir string
}

// Note: Where possible, we have flag default values be zero values, so that
// struct-based configuration matches flag-based configuration.
func (o *Opts) InitFlags(f *flag.FlagSet) {
	f.StringVar(&o.MountName, "name", "", `If provided, causes the mount table to mount itself under this name.  The name may be absolute for a remote mount table service (e.g. "/<remote mt address>//some/suffix") or could be relative to this process' default mount table (e.g. "some/suffix").`)
	f.StringVar(&o.AclFile, "acls", "", "ACL file.  Default is to allow all access.")
	f.StringVar(&o.NhName, "neighborhood-name", "", "If provided, enables sharing with the local neighborhood with the provided name.  The address of this mount table will be published to the neighboorhood and everything in the neighborhood will be visible on this mount table.")
	f.StringVar(&o.PersistDir, "persist-dir", "", "Directory in which to persist permissions.")
}
