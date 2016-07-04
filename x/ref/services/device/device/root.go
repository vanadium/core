// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The following enables go generate to generate the doc.go file.
//go:generate go run $JIRI_ROOT/release/go/src/v.io/x/lib/cmdline/testdata/gendoc.go .

package main

import (
	"regexp"

	"v.io/x/lib/cmdline"
	_ "v.io/x/ref/runtime/factories/roaming"
)

var CmdRoot = &cmdline.Command{
	Name:  "device",
	Short: "facilitates interaction with the Vanadium device manager",
	Long: `
Command device facilitates interaction with the Vanadium device manager.
`,
	Children: []*cmdline.Command{cmdInstall, cmdInstallLocal, cmdUninstall, cmdAssociate, cmdDescribe, cmdClaim, cmdInstantiate, cmdDelete, cmdRun, cmdKill, cmdRevert, cmdUpdate, cmdStatus, cmdDebug, cmdACL, cmdPublish, cmdLs},
}

func main() {
	cmdline.HideGlobalFlagsExcept(regexp.MustCompile(`^((v23\.namespace\.root)|(v23\.proxy))$`))
	cmdline.Main(CmdRoot)
}
