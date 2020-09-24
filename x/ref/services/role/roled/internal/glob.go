// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"os"
	"path/filepath"
	"strings"

	"v.io/v23/context"
	"v.io/v23/glob"
	"v.io/v23/naming"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/verror"
)

func globChildren(ctx *context.T, call rpc.GlobChildrenServerCall, serverConfig *serverConfig, m *glob.Element) error {
	sCall := call.Security()
	n := findRoles(ctx, sCall, serverConfig.root)
	suffix := sCall.Suffix()
	if len(suffix) > 0 {
		n = n.find(strings.Split(suffix, "/"), false)
	}
	if n == nil {
		return verror.ErrNoExistOrNoAccess.Errorf(ctx, "does not exist or access denied")
	}
	for c := range n.children {
		if m.Match(c) {
			//nolint:errcheck
			call.SendStream().Send(naming.GlobChildrenReplyName{Value: c})
		}
	}
	return nil
}

// findRoles finds all the roles to which the caller has access.
func findRoles(ctx *context.T, call security.Call, root string) *node {
	blessingNames, _ := security.RemoteBlessingNames(ctx, call)
	tree := newNode()
	//nolint:errcheck
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".conf") {
			return nil
		}
		c, err := loadExpandedConfig(path, nil)
		if err != nil {
			return nil
		}
		if !hasAccess(c, blessingNames) {
			return nil
		}
		relPath, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		tree.find(strings.Split(strings.TrimSuffix(relPath, ".conf"), string(filepath.Separator)), true)
		return nil
	})
	return tree
}

type node struct {
	children map[string]*node
}

func newNode() *node {
	return &node{children: make(map[string]*node)}
}

func (n *node) find(names []string, create bool) *node {
	for {
		if len(names) == 0 {
			return n
		}
		if next, ok := n.children[names[0]]; ok {
			n = next
			names = names[1:]
			continue
		}
		if create {
			nn := newNode()
			n.children[names[0]] = nn
			n = nn
			names = names[1:]
			continue
		}
		return nil
	}
}
