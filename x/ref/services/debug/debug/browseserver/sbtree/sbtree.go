// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package sbtree provides data structures used by HTML templates to build the
// web pages for the Syncbase debug viewer.  To minimize mixing of code and
// presentation, all the data for use in the templates is in a form that is
// convenient for accessing and iterating over, i.e struct fields, no-arg
// methods, slices, or maps.  In some cases this required mirroring data
// structures in the public Syncbase API to avoid having the templates deal with
// context objects, or to avoid the templates needing extra variables to handle
// indirection.
package sbtree

import (
	"fmt"

	"v.io/v23/context"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/syncbase"
)

// SyncbaseTree has all the data for the main page of the Syncbase debug viewer.
type SyncbaseTree struct {
	Service syncbase.Service
	Dbs     []dbTree
}

type dbTree struct {
	Database    syncbase.Database
	Collections []syncbase.Collection
	Syncgroups  []syncgroupTree
	Errs        []error
}

type syncgroupTree struct {
	Syncgroup syncbase.Syncgroup
	Spec      wire.SyncgroupSpec
	Members   map[string]wire.SyncgroupMemberInfo
	Errs      []error
}

// AssembleSyncbaseTree returns information describing the Syncbase server
// running on the given server. Errors are included in the tree, so they can be
// displayed in the HTML.
func AssembleSyncbaseTree(
	ctx *context.T, server string, service syncbase.Service, dbIds []wire.Id,
) *SyncbaseTree {
	dbTrees := make([]dbTree, len(dbIds))
	for i := range dbIds {
		dbTrees[i] = dbTree{}

		// TODO(eobrain) Confirm nil for schema is appropriate
		db := service.DatabaseForId(dbIds[i], nil)
		dbTrees[i].Database = db

		// Assemble collections
		collIds, err := db.ListCollections(ctx)
		if err == nil {
			dbTrees[i].Collections = make([]syncbase.Collection, len(collIds))
			for j := range collIds {
				dbTrees[i].Collections[j] = db.CollectionForId(collIds[j])
			}
		} else {
			dbTrees[i].Errs = append(dbTrees[i].Errs,
				fmt.Errorf("Problem listing collections: %v", err))
		}

		// Assemble syncgroups
		sgIds, err := db.ListSyncgroups(ctx)
		if err != nil {
			dbTrees[i].Errs = append(dbTrees[i].Errs, err)
			continue
		}
		sgs := make([]syncgroupTree, len(sgIds))
		dbTrees[i].Syncgroups = sgs
		for j := range sgIds {
			sgs[j] = syncgroupTree{}
			sg := db.SyncgroupForId(sgIds[j])
			sgs[j].Syncgroup = sg
			spec, _, err := sg.GetSpec(ctx)
			if err != nil {
				sgs[j].Errs = append(sgs[j].Errs,
					fmt.Errorf("Problem getting spec of syncgroup: %v", err))
				continue
			}
			sgs[j].Spec = spec
			members, err := sg.GetMembers(ctx)
			if err != nil {
				sgs[j].Errs = append(sgs[j].Errs,
					fmt.Errorf("Problem getting members of syncgroup: %v", err))
				continue
			}
			sgs[j].Members = members
		}
	}

	return &SyncbaseTree{service, dbTrees}
}
