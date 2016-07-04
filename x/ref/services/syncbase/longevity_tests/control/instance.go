// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package control

import (
	"fmt"
	"os"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/security"
	"v.io/x/lib/gosh"
	"v.io/x/ref/services/syncbase/longevity_tests/client"
	"v.io/x/ref/services/syncbase/longevity_tests/model"
	"v.io/x/ref/services/syncbase/longevity_tests/syncbased_vine"
	"v.io/x/ref/services/syncbase/syncbaselib"
)

var (
	syncbasedMain = gosh.RegisterFunc("syncbasedMain", syncbased_vine.Main)
)

// instance encapsulates a running syncbase instance.
type instance struct {
	// Name of the instance.  Syncbased will be mounted under this name.
	name string

	// Databases to host on the syncbased.
	databases model.DatabaseSet

	// Context used by clients.
	clientCtx *context.T

	// Clients of the syncbase instance.
	clients []client.Client

	// Gosh command for syncbase process.  Will be nil if instance is not
	// running.
	cmd *gosh.Cmd

	// Directory containing v23 credentials for this instance.
	credsDir string

	// Namespace root.
	namespaceRoot string

	// Principal for syncbase instance.
	principal security.Principal

	// Gosh shell used to spawn all processes.
	sh *gosh.Shell

	// Name of the vine server and tag running on instance.
	vineName string

	// Working directory for this instance.
	wd string
}

func (inst *instance) start(rootCtx *context.T) error {
	if inst.cmd != nil {
		return fmt.Errorf("inst %v already started", inst)
	}
	opts := syncbaselib.Opts{
		Name:    inst.name,
		RootDir: inst.wd,
		// TODO(nlacasse): Turn this on once neighborhoods can be configured via VINE.
		SkipPublishInNh: false,
	}
	// We use the vine name as both the vine server name and the tag.
	inst.cmd = inst.sh.FuncCmd(syncbasedMain, inst.vineName, inst.vineName, opts)
	// TODO(nlacasse): Once we have user credentials, only allow blessings
	// based on the user here.
	perms := `{"Admin":{"In":["root"],"NotIn":null},"Debug":{"In":["root"],"NotIn":null},"Read":{"In":["root"],"NotIn":null},"Resolve":{"In":["root"],"NotIn":null},"Write":{"In":["root"],"NotIn":null}}`
	inst.cmd.Args = append(inst.cmd.Args,
		"--log_dir="+inst.wd,
		"--v23.namespace.root="+inst.namespaceRoot,
		"--v23.credentials="+inst.credsDir,
		"--v23.permissions.literal="+perms,
		"--vpath=vsync=5",
	)
	inst.cmd.Start()
	vars := inst.cmd.AwaitVars("ENDPOINT")
	if ep := vars["ENDPOINT"]; ep == "" {
		return fmt.Errorf("error starting %q: no ENDPOINT variable sent from process", inst.name)
	}

	// Derive a context with the user's principal from rootCtx.
	clientCtx, err := v23.WithPrincipal(rootCtx, inst.principal)
	if err != nil {
		return err
	}
	inst.clientCtx = clientCtx

	// Start clients.
	return inst.startClients()
}

func (inst *instance) update(d *model.Device) error {
	return fmt.Errorf("not implemented")
}

func (inst *instance) startClients() error {
	for _, client := range inst.clients {
		client.Start(inst.clientCtx, inst.name, inst.databases)
	}
	return nil
}

func (inst *instance) stopClients() error {
	for _, client := range inst.clients {
		if err := client.Stop(); err != nil {
			return err
		}
	}
	return nil
}

func (inst *instance) stopSyncbase() error {
	inst.cmd.Terminate(os.Interrupt)
	inst.cmd = nil
	return nil
}
