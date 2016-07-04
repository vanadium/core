// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package util

import (
	"sort"
	"strings"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/naming"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/verror"
)

// byId implements sort.Interface for []wire.Id. Sorts by blessing, then name.
type byId []wire.Id

func (a byId) Len() int      { return len(a) }
func (a byId) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a byId) Less(i, j int) bool {
	if a[i].Blessing != a[j].Blessing {
		return a[i].Blessing < a[j].Blessing
	}
	return a[i].Name < a[j].Name
}

// SortIds sorts a list of ids by blessing, then name.
func SortIds(ids []wire.Id) {
	sort.Sort(byId(ids))
}

// ListChildIds returns a sorted list of ids of all children of parentFullName.
func ListChildIds(ctx *context.T, parentFullName string) ([]wire.Id, error) {
	ns := v23.GetNamespace(ctx)
	ch, err := ns.Glob(ctx, naming.Join(parentFullName, "*"))
	if err != nil {
		return nil, err
	}
	ids := []wire.Id{}
	for reply := range ch {
		switch v := reply.(type) {
		case *naming.GlobReplyEntry:
			encId := v.Value.Name[strings.LastIndex(v.Value.Name, "/")+1:]
			// Component ids within object names are always encoded. See comment in
			// server/dispatcher.go for explanation.
			id, err := DecodeId(encId)
			if err != nil {
				// If this happens, there's a bug in the Syncbase server. Glob should
				// return names with escaped components.
				return nil, verror.New(verror.ErrInternal, ctx, err)
			}
			ids = append(ids, id)
		case *naming.GlobReplyError:
			// TODO(sadovsky): Surface these errors somehow.
		}
	}
	SortIds(ids)
	return ids, nil
}
