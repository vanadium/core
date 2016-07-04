// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package commands

import (
	"bytes"
	"fmt"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/security"
	"v.io/v23/security/access"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/syncbase"
	"v.io/v23/syncbase/util"
	"v.io/x/lib/cmdline"
)

var cmdSg = &cmdline.Command{
	Name:     "sg",
	Short:    "Manipulate syncgroups",
	Long:     "Manipulate syncgroups.",
	Children: []*cmdline.Command{cmdSgCreate, cmdSgList, cmdSgInspect, cmdSgJoin, cmdSgLeave, cmdSgDestroy, cmdSgEject},
}

var (
	flagSgTarget      string
	flagSgDescription string
	flagSgPerms       string
	flagSgListVerbose bool
)

func init() {
	cmdSg.Flags.StringVar(&flagSgTarget, "target", "", "Syncgroup to act on.")
	cmdSg.Flags.StringVar(&flagSgTarget, "t", "", "Syncgroup to act on.")
	cmdSgCreate.Flags.StringVar(&flagSgDescription, "description", "", "Description for the new syncgroup.")
	cmdSgCreate.Flags.StringVar(&flagSgPerms, "perms", "", `
Permissions for the new syncgroup.
The permissions format for syncgroups is:
{<tag>:{"In":[<blessing_pattern>,...],"Out":[<blessing_pattern>,...]},...}
where the <tag>s Admin and Read must exist. The specified permissions must
include at least one Admin.
`)
	cmdSgList.Flags.BoolVar(&flagSgListVerbose, "verbose", false, "Show entire id of syncgroups and blessings")
	cmdSgList.Flags.BoolVar(&flagSgListVerbose, "v", false, "Show entire id of syncgroups and blessings")
}

var cmdSgCreate = &cmdline.Command{
	Name:     "create",
	Short:    "Create a new syncgroup with the given spec",
	Long:     "Create a new syncgroup with the given spec.",
	ArgsName: "<collection_id>[ <collection_id>]*",
	ArgsLong: `
Takes a space separated list of collection ids where each collection is covered
by this syncgroup.

The -perms flag can be used to specify a specific set of permissions.
Alternatively, leave the perms flag absent for default permissions (principal has
Admin and Read) and use the acl command to adjust these as desired.
`,
	Runner: SbRunner(runSgCreate),
}

func runSgCreate(ctx *context.T, db syncbase.Database, env *cmdline.Env, args []string) error {
	var spec wire.SyncgroupSpec
	spec.Description = flagSgDescription

	// Get collection ids for specified collections.
	spec.Collections = make([]wire.Id, len(args))
	for i, name := range args {
		if id, err := wire.ParseId(name); err != nil {
			return env.UsageErrorf("create: error parsing id %s: %q", name, err)
		} else {
			spec.Collections[i] = id
		}
	}

	// TODO(zsterling): Allow users to override default blessing.
	if bp, err := getDefaultBlessing(ctx); err != nil {
		return err
	} else if flagSgPerms == "" {
		// Default perms.
		spec.Perms = map[string]access.AccessList{
			"Admin": access.AccessList{[]security.BlessingPattern{bp}, []string{}},
			"Read":  access.AccessList{[]security.BlessingPattern{bp}, []string{}},
		}
	} else {
		// Manually set perms by flag.
		if perms, err := access.ReadPermissions(bytes.NewBufferString(flagSgPerms)); err != nil {
			return env.UsageErrorf("create: bad permissions: %q", err)
		} else {
			spec.Perms = perms
		}
	}

	sg := db.Syncgroup(ctx, flagSgTarget)
	if err := sg.Create(ctx, spec, wire.SyncgroupMemberInfo{}); err != nil {
		return fmt.Errorf("create: error creating syncgroup: %q", err)
	}

	return nil
}

func getDefaultBlessing(ctx *context.T) (security.BlessingPattern, error) {
	principal := v23.GetPrincipal(ctx)
	blessings := security.DefaultBlessingNames(principal)
	_, user, err := util.AppAndUserPatternFromBlessings(blessings...)
	return user, err
}

var cmdSgList = &cmdline.Command{
	Name:   "list",
	Short:  "List all syncgroups on the database",
	Long:   "List all syncgroups on the database.",
	Runner: SbRunner(runSgList),
}

func runSgList(ctx *context.T, db syncbase.Database, _ *cmdline.Env, _ []string) error {
	ids, err := db.ListSyncgroups(ctx)
	if err != nil {
		return err
	}

	collections := make(map[wire.Id][]wire.Id)
	for _, id := range ids {
		sg := db.SyncgroupForId(id)
		var spec wire.SyncgroupSpec
		spec, _, err = sg.GetSpec(ctx)
		if err != nil {
			return err
		}
		collections[id] = spec.Collections
	}

	for i, id := range ids {
		if flagSgListVerbose {
			fmt.Println(id)
		} else {
			fmt.Println(id.Name)
		}
		for _, coll := range collections[id] {
			if flagSgListVerbose {
				fmt.Println("\t", coll)
			} else {
				fmt.Println("\t", coll.Name)
			}
		}
		if i < len(ids)-1 {
			fmt.Println()
		}
	}
	return nil
}

var cmdSgInspect = &cmdline.Command{
	Name:  "inspect",
	Short: "Show spec of a syncgroup",
	Long: `
Show spec of the syncgroup specified by the -target flag.
`,
	Runner: SbRunner(runSgInspect),
}

func runSgInspect(ctx *context.T, db syncbase.Database, _ *cmdline.Env, _ []string) error {
	sg := db.Syncgroup(ctx, flagSgTarget)
	spec, _, err := sg.GetSpec(ctx)
	if err != nil {
		return err
	}

	fmt.Println("Description:\t\t", spec.Description)
	fmt.Println("PublishSyncbaseName:\t", spec.PublishSyncbaseName)
	fmt.Println("Perms:\t\t\t", spec.Perms)
	fmt.Println("Collections:\t\t", spec.Collections)
	fmt.Println("MountTables:\t\t", spec.MountTables)
	fmt.Println("IsPrivate:\t\t", spec.IsPrivate)

	return nil
}

var cmdSgJoin = &cmdline.Command{
	Name:     "join",
	Short:    "Join specified syncgroup",
	Long:     "Join specified syncgroup.",
	ArgsName: "[<admin>]",
	ArgsLong: `
<admin> is the name of a syncbase instance which is an Admin on the syncgroup.
If <admin> is not specified join will look for syncgroups on the local neighborhood.
`,
	Runner: SbRunner(runSgJoin),
}

func runSgJoin(ctx *context.T, db syncbase.Database, env *cmdline.Env, args []string) error {
	var adminName string
	switch len(args) {
	case 0:
		adminName = ""
	case 1:
		adminName = args[0]
	default:
		return env.UsageErrorf("join: too many args")
	}

	sg := db.Syncgroup(ctx, flagSgTarget)
	// TODO(zsterling): Pass in expected blessings.
	_, err := sg.Join(ctx, adminName, nil, wire.SyncgroupMemberInfo{})
	return err
}

var cmdSgLeave = &cmdline.Command{
	Name:   "leave",
	Short:  "Leave specified syncgroup",
	Long:   "Leave specified syncgroup.",
	Runner: SbRunner(runSgLeave),
}

func runSgLeave(ctx *context.T, db syncbase.Database, _ *cmdline.Env, _ []string) error {
	sg := db.Syncgroup(ctx, flagSgTarget)
	return sg.Leave(ctx)
}

var cmdSgDestroy = &cmdline.Command{
	Name:   "destroy",
	Short:  "Destroy specified syncgroup",
	Long:   "Destroy specified syncgroup.",
	Runner: SbRunner(runSgDestroy),
}

func runSgDestroy(ctx *context.T, db syncbase.Database, _ *cmdline.Env, _ []string) error {
	sg := db.Syncgroup(ctx, flagSgTarget)
	return sg.Destroy(ctx)
}

var cmdSgEject = &cmdline.Command{
	Name:     "eject",
	Short:    "Eject user from syncgroup",
	Long:     "Eject user from syncgroup.",
	ArgsName: "<user>",
	// TODO(zsterling): Ensure user should really be a blessing when the api is
	// implemented.
	ArgsLong: `
<user> should be the blessing of the user to eject.
`,
	Runner: SbRunner(runSgEject),
}

func runSgEject(ctx *context.T, db syncbase.Database, env *cmdline.Env, args []string) error {
	if got := len(args); got != 1 {
		return env.UsageErrorf("eject: expected 1 argument, got %d", got)
	}

	sg := db.Syncgroup(ctx, flagSgTarget)
	return sg.Eject(ctx, args[0])
}
