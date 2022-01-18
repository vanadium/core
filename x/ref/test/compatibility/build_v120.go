// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package compatibility

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"v.io/v23/context"
)

func majorMinor(version, prefix string) (string, bool) {
	if !strings.HasPrefix(version, prefix) {
		return "", false
	}
	if version == prefix {
		return "", true
	}
	if at := strings.Index(version, "@"); at > 0 {
		return version[at:], true
	}
	return "", false
}

// BuildSimpleAtVersion builds v.io/x/ref/test/compatibility/modules/simple
// at the specified version. Currently v0.1.20 is the only supported version.
// If bindir is specified then the binaries are written to that directory.
func BuildSimpleAtVersion(ctx *context.T, version, bindir string, verbose bool) (client, server, mt string, cleanup func(), err error) {
	if hash, ok := majorMinor(version, "v0.1.20"); ok {
		return buildSimpleV120(ctx, hash, bindir, verbose)
	}
	return "", "", "", func() {}, fmt.Errorf("unsupported version: %v", version)

}

func buildSimpleV120(ctx *context.T, version, bindir string, verbose bool) (client, server, mt string, cleanup func(), err error) {
	cleanup = func() {}
	tmpDir, err := os.MkdirTemp("", "backwards-compat")
	if err != nil {
		return
	}
	if len(bindir) > 0 {
		client = filepath.Join(bindir, "v120-simple-client")
		server = filepath.Join(bindir, "v120-simple-server")
		mt = filepath.Join(bindir, "v120-mounttabled")
	}
	pkg := "v.io/x/ref/test/compatibility/modules/simple"
	if len(version) > 0 {
		pkg += version
	}
	_, binaries, cleanupBuild, err := BuildWithDependencies(ctx,
		pkg,
		GOPATH(tmpDir),
		Build("client", client),
		Build("server", server),
		Build("mounttabled", mt),
		Verbose(verbose),
		GoMod("edit", "--go=1.13"),
		GoMod("edit", "--dropreplace=v.io"),
		GoMod("edit", "--require=v.io@v0.1.20"),
		GoMod("edit", "--exclude=github.com/shirou/gopsutil@v2.19.9+incompatible"),
		GoMod("tidy"),
	)
	cleanup = func() {
		os.RemoveAll(tmpDir)
		cleanupBuild()
	}
	if err != nil {
		return "", "", "", cleanup, err
	}
	client, server, mt = binaries[0], binaries[1], binaries[2]
	return
}
