// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"google.golang.org/cloud/bigtable"

	"v.io/v23/naming"
	"v.io/v23/security"
	"v.io/v23/security/access"
	"v.io/x/ref/test"
)

func TestMutations(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	bt, shutdownBT, err := NewTestBigTable("test")
	if err != nil {
		t.Fatalf("NewTestBigTable: %s", err)
	}
	defer shutdownBT()
	if err := bt.SetupTable(ctx, ""); err != nil {
		t.Fatalf("bt.SetupTable: %s", err)
	}

	root, err := getNode(ctx, bt, "")
	if err != nil {
		t.Fatalf("getNode: %v", err)
	}

	ops := []func(i int, n *mtNode) bool{
		func(i int, n *mtNode) bool {
			if err := n.delete(ctx, false); err != nil {
				t.Logf("[%d] delete: %v", i, err)
				return false
			}
			t.Logf("[%d] delete succeeded", i)
			m, err := getNode(ctx, bt, "X")
			if err != nil {
				t.Errorf("[%d] getNode failed after delete: %v", i, err)
				return false
			}
			if m != nil {
				t.Errorf("[%d] node still exists after delete", i)
				return false
			}
			return true
		},
		func(i int, n *mtNode) bool {
			if child, err := n.createChild(ctx, "Y", n.permissions, "", 0); err != nil {
				t.Logf("[%d] createChild: (%v, %v)", i, child, err)
				return false
			}
			t.Logf("[%d] createChild succeeded", i)
			m, err := getNode(ctx, bt, "X")
			if err != nil {
				t.Errorf("[%d] getNode failed after createChild: %v", i, err)
				return false
			}
			if m == nil {
				t.Errorf("[%d] node doesn't exist after createChild", i)
				return false
			}
			if len(m.children) != 1 || m.children[0].name() != "Y" {
				t.Errorf("[%d] unexpected children after createChild: %v", i, m.children)
				return false
			}
			return true
		},
		func(i int, n *mtNode) bool {
			const serverAddr = "/example.com:1234"
			if err := n.mount(ctx, serverAddr, clock.Now().Add(time.Minute), 0, 0); err != nil {
				t.Logf("[%d] mount: %v", i, err)
				return false
			}
			t.Logf("[%d] mount succeeded", i)
			m, err := getNode(ctx, bt, "X")
			if err != nil {
				t.Errorf("[%d] getNode failed after mount: %v", i, err)
				return false
			}
			if m == nil {
				t.Errorf("[%d] node doesn't exist after mount", i)
				return false
			}
			if len(m.servers) != 1 || m.servers[0].Server != serverAddr {
				t.Errorf("[%d] unexpected servers after mount: %v", i, m.servers)
				return false
			}
			return true
		},
		func(i int, n *mtNode) bool {
			perm := access.Permissions{"Admin": access.AccessList{In: []security.BlessingPattern{"foo"}}}
			if err := n.setPermissions(ctx, perm); err != nil {
				t.Logf("[%d] setPermissions: %v", i, err)
				return false
			}
			t.Logf("[%d] setPermissions succeeded", i)
			m, err := getNode(ctx, bt, "X")
			if err != nil {
				t.Errorf("[%d] getNode failed after setPermissions: %v", i, err)
				return false
			}
			if m == nil {
				t.Errorf("[%d] node doesn't exist after setPermissions", i)
				return false
			}
			if len(m.permissions["Admin"].In) != 1 || string(m.permissions["Admin"].In[0]) != "foo" {
				t.Errorf("[%d] unexpected permissions after setPermissions: %v", i, m.permissions)
			}
			return true
		},
	}
	for i := 1; i <= 100; i++ {
		// Create X.
		root, _ = getNode(ctx, bt, "")
		n, err := root.createChild(ctx, "X", root.permissions, "", 0)
		if err != nil || n == nil {
			t.Fatalf("createChild(X): (%v, %v)", n, err)
		}

		// Attempt all the mutations in random order. Only the first one
		// should ever succeed because the node version changes after
		// successful mutations.
		for j, p := range rand.Perm(len(ops)) {
			success := ops[p](i, n)
			switch {
			case j == 0 && !success:
				t.Errorf("Unexpected failure for first mutation")
			case j > 0 && success:
				t.Errorf("Unexpected success for mutation")
			}
		}

		// Delete X, if it still exists.
		if n, err := getNode(ctx, bt, "X"); err == nil && n != nil {
			n.delete(ctx, true)
		}
	}

	count, err := bt.CountRows(ctx)
	if err != nil {
		t.Errorf("CountRows failed: %v", err)
	}
	if expected := 1; count != expected {
		t.Errorf("Unexpected number of rows: got %d, expected %d", count, expected)
		bt.DumpTable(ctx)
	}
}

func TestFsck(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	bt, shutdownBT, err := NewTestBigTable("test")
	if err != nil {
		t.Fatalf("NewTestBigTable: %s", err)
	}
	defer shutdownBT()
	if err := bt.SetupTable(ctx, ""); err != nil {
		t.Fatalf("bt.SetupTable: %s", err)
	}

	checkRowCount := func(expected int) {
		count, err := bt.CountRows(ctx)
		if err != nil {
			t.Errorf("CountRows failed: %v", err)
		}
		if count != expected {
			t.Errorf("Unexpected number of rows: got %d, expected %d", count, expected)
		}
	}

	root, err := getNode(ctx, bt, "")
	if err != nil {
		t.Fatalf("getNode: %v", err)
	}

	// Create a few children.
	for i := 1; i <= 5; i++ {
		if _, err := root.createChild(ctx, fmt.Sprintf("child%d", i), root.permissions, "", 0); err != nil {
			t.Fatalf("createChild failed: %v", err)
		}
		if root, err = getNode(ctx, bt, ""); err != nil {
			t.Fatalf("getNode: %v", err)
		}
	}
	checkRowCount(6)

	// Create a few orphan rows.
	for i := 1; i <= 5; i++ {
		name := fmt.Sprintf("orphan%d", i)
		childRef, err := newChild("XXXXXXXX", name)
		if err != nil {
			t.Fatalf("newChild failed: %v", err)
		}
		if err := bt.createRow(ctx, name, root.permissions, "", childRef, 0); err != nil {
			t.Fatalf("createRow failed: %v", err)
		}
		if childRef, err = newChild("XXXXXXXX", "otherorphan"); err != nil {
			t.Fatalf("newChild failed: %v", err)
		}
		if err := bt.createRow(ctx, naming.Join(name, childRef.name()), root.permissions, "", childRef, 0); err != nil {
			t.Fatalf("createRow failed: %v", err)
		}
		for j := 1; j <= 5; j++ {
			n, err := getNode(ctx, bt, name)
			if err != nil {
				t.Fatalf("getNode: %v", err)
			}
			if _, err := n.createChild(ctx, fmt.Sprintf("child%d", j), n.permissions, "", 0); err != nil {
				t.Fatalf("createChild failed: %v", err)
			}
		}
	}
	checkRowCount(41)

	// Add reference to a row that doesn't exist.
	mut := bigtable.NewMutation()
	mut.Set(childrenFamily, "XXXXXXXXwhodat", bigtable.ServerTime, []byte{1})
	if err := bt.apply(ctx, rowKey("child1"), mut); err != nil {
		t.Fatalf("unexpected failure: %v", err)
	}
	checkRowCount(41)

	if err := bt.Fsck(ctx, false); err == nil {
		t.Errorf("Fsck did not find any inconsistencies")
	}
	if err := bt.Fsck(ctx, true); err != nil {
		t.Errorf("Fsck failed: %v", err)
	}
	if err := bt.Fsck(ctx, false); err != nil {
		t.Errorf("Fsck failed: %v", err)
	}

	// root + 5 children
	checkRowCount(6)
}
