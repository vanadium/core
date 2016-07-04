// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package commands

import (
	"fmt"
	"strings"

	"v.io/v23/context"
	"v.io/v23/security"
	"v.io/v23/security/access"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/syncbase"
	sbUtil "v.io/v23/syncbase/util"
	"v.io/x/lib/cmdline"
	"v.io/x/ref/cmd/sb/dbutil"
)

var cmdAcl = &cmdline.Command{
	Name:     "acl",
	Short:    "Read or mutate the ACLs",
	Long:     `Read or mutate the ACLs.`,
	Children: []*cmdline.Command{cmdAclGet, cmdAclAdd, cmdAclRm, cmdAclDump},
}

var (
	flagTarget     string
	flagCollection string
	flagSyncgroup  string
)

func init() {
	cmdAcl.Flags.StringVar(&flagTarget, "target", "db", "The access controller type to act on (service, db, database, cx, collection, sg, syncgroup).")
	cmdAcl.Flags.StringVar(&flagCollection, "cx", "", "The collection to act on, in format <blessing>,<collection>. (note: requires -target=collection or cx)")
	cmdAcl.Flags.StringVar(&flagSyncgroup, "sg", "", "The syncgroup to act on, in format <blessing>,<syncgroup>. (note: requires -target=syncgroup or sg)")
}

func updatePerms(ctx *context.T, db syncbase.Database, env *cmdline.Env, callback func(access.Permissions) (access.Permissions, error)) error {
	switch flagTarget {
	case "service", "db", "database":
		var ac sbUtil.AccessController
		if flagTarget == "service" {
			ac = dbutil.GetService()
		} else {
			ac = db
		}

		perms, version, err := ac.GetPermissions(ctx)
		if err != nil {
			return err
		}

		if perms, err = callback(perms); err != nil {
			return err
		} else if perms == nil {
			return nil
		}

		if err := ac.SetPermissions(ctx, perms, version); err != nil {
			return fmt.Errorf("error setting permissions: %q", err)
		}
		return nil
	case "cx", "collection":
		bdb, err := db.BeginBatch(ctx, wire.BatchOptions{})
		if err != nil {
			return err
		}
		defer bdb.Abort(ctx)

		id, err := wire.ParseId(flagCollection)
		if err != nil {
			return err
		}
		collection := bdb.CollectionForId(id)
		perms, err := collection.GetPermissions(ctx)
		if err != nil {
			return err
		}

		if perms, err = callback(perms); err != nil {
			return err
		} else if perms == nil {
			return nil
		}

		if err = collection.SetPermissions(ctx, perms); err != nil {
			return err
		}

		return bdb.Commit(ctx)
	case "sg", "syncgroup":
		id, err := wire.ParseId(flagSyncgroup)
		if err != nil {
			return err
		}
		sg := db.SyncgroupForId(id)
		spec, version, err := sg.GetSpec(ctx)
		if err != nil {
			return err
		}

		if spec.Perms, err = callback(spec.Perms); err != nil {
			return err
		} else if spec.Perms == nil {
			return nil
		}

		sg.SetSpec(ctx, spec, version)
		return nil
	default:
		return env.UsageErrorf("target %s not recognized", flagTarget)
	}

	return nil
}

var cmdAclGet = &cmdline.Command{
	Name:     "get",
	Short:    "Read a blessing's permissions",
	Long:     "Read a blessing's permissions.",
	ArgsName: "<blessing>",
	ArgsLong: `
<blessing> is the blessing to check permissions for.
`,
	Runner: SbRunner(runAclGet),
}

func runAclGet(ctx *context.T, db syncbase.Database, env *cmdline.Env, args []string) error {
	if got := len(args); got != 1 {
		return env.UsageErrorf("get: expected 1 arg, got %d", got)
	}

	blessing := args[0]
	return updatePerms(ctx, db, env,
		func(perms access.Permissions) (access.Permissions, error) {
			// Pretty-print the groups that blessing is in.
			var groupsIn []string
			for groupName, acl := range perms {
				for _, b := range acl.In {
					if b == security.BlessingPattern(blessing) {
						groupsIn = append(groupsIn, groupName)
						break
					}
				}
			}
			fmt.Printf("[%s]\n", strings.Join(groupsIn, ", "))

			// Pretty-print the groups that blessing is blacklisted from.
			var groupsNotIn []string
			for groupName, acl := range perms {
				for _, b := range acl.NotIn {
					if b == blessing {
						groupsNotIn = append(groupsNotIn, groupName)
						break
					}
				}
			}
			fmt.Printf("![%s]\n", strings.Join(groupsNotIn, ", "))

			return nil, nil
		},
	)
}

var cmdAclAdd = &cmdline.Command{
	Name:     "add",
	Short:    "Add blessing to groups",
	Long:     "Add blessing to groups.",
	ArgsName: "(<blessing> [!]<tag>(,[!]<tag>)* )+",
	ArgsLong: `
The args are sequential pairs of the form "<blessing> <taglist>"
A taglist is made up of a series of comma-separated tags.
The optional preface "!" puts the blessing in the blacklist instead of the whitelist.
Note that the pair "dev.v.io:u:foo@google.com bar,baz,!bar" is legal.
In such a case the blessing will be added to both the blacklist and the whitelist,
with the blacklist overriding the whitelist.
`,
	Runner: SbRunner(runAclAdd),
}

func runAclAdd(ctx *context.T, db syncbase.Database, env *cmdline.Env, args []string) error {
	if len(args) == 0 {
		return env.UsageErrorf("add: expected arguments")
	}

	userToPerms, err := parseAccessList(args)
	if err != nil {
		return env.UsageErrorf("add: error parsing access list: %q", err)
	}

	return updatePerms(ctx, db, env,
		func(perms access.Permissions) (access.Permissions, error) {
			for blessing, tags := range userToPerms {
				bp := security.BlessingPattern(blessing)
				perms = perms.Add(bp, tags.in...)
				perms = perms.Blacklist(blessing, tags.out...)
			}
			return perms, nil
		},
	)
}

var cmdAclRm = &cmdline.Command{
	Name:     "rm",
	Short:    "Remove specific permissions from the acl",
	Long:     "Remove specific permissions from the acl.",
	ArgsName: "(<blessing> <tag>(,<tag>)* )+",
	ArgsLong: `
The args are sequential pairs of the form "<blessing> <taglist>"
A taglist is made up of a series of comma-separated tags.
rm removes all references to each <blessing> from all of the tags.
`,
	Runner: SbRunner(runAclRm),
}

func runAclRm(ctx *context.T, db syncbase.Database, env *cmdline.Env, args []string) error {
	if len(args) == 0 {
		return env.UsageErrorf("rm: expected arguments")
	}

	userToPerms, err := parseAccessList(args)
	if err != nil {
		return env.UsageErrorf("rm: failed to parse access list: %q", err)
	}

	return updatePerms(ctx, db, env,
		func(perms access.Permissions) (access.Permissions, error) {
			for blessing, tags := range userToPerms {
				perms = perms.Clear(blessing, tags.in...)
			}
			return perms, nil
		},
	)
}

var cmdAclDump = &cmdline.Command{
	Name:   "dump",
	Short:  "Pretty-print the whole acl",
	Long:   "Pretty-print the whole acl.",
	Runner: SbRunner(runAclDump),
}

func runAclDump(ctx *context.T, db syncbase.Database, env *cmdline.Env, args []string) error {
	return updatePerms(ctx, db, env,
		func(perms access.Permissions) (access.Permissions, error) {
			fmt.Println("map[")
			for tag, acl := range perms {
				fmt.Printf("\t%s: %v\n", tag, acl)
			}
			fmt.Println("]")
			return nil, nil
		},
	)
}

type userTags struct {
	in  []string
	out []string
}

// parseAccessList returns a map from blessing patterns to structs containing lists
// of tags which the pattern is white/blacklisted on.
func parseAccessList(args []string) (map[string]userTags, error) {
	if got := len(args); got%2 != 0 {
		return nil, fmt.Errorf("incorrect number of arguments %d, must be even", got)
	}

	userToTags := make(map[string]userTags)
	for i := 0; i < len(args); i += 2 {
		tags, err := parseAccessTags(args[i+1])
		if err != nil {
			return nil, err
		}
		userToTags[args[i]] = tags
	}

	return userToTags, nil
}

// parseAccessTags returns two lists of tags, the first being tags which have
// whitelisted the blessing, and the second being tags which have blacklisted it.
func parseAccessTags(input string) (tags userTags, err error) {
	fields := strings.Split(input, ",")
	for _, tag := range fields {
		blacklist := strings.HasPrefix(tag, "!")
		if blacklist {
			tag = tag[1:]
		}
		if len(tag) == 0 {
			return userTags{}, fmt.Errorf("empty access tag in %q", input)
		}

		if blacklist {
			tags.out = append(tags.out, tag)
		} else {
			tags.in = append(tags.in, tag)
		}
	}

	return
}
