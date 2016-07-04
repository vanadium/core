// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package ptrie provides a ptrie to store a mapping from bit strings to
// arbitrary values. Ptrie exposes a simple interface: Get(key),
// Put(key, value), Delete(key), and Scan(start, limit). Conceptually a ptrie
// can be thought of as a map[[]byte]interface{}, designed to support fast
// range queries and immutable views.
//
// For performance reasons, bit strings are represented as byte slices.
// The ptrie consists of three concepts: a binary trie, path contraction and
// copy-on-write modifications.
//
// 1) A binary trie consists of nodes and arcs. Each node has a value and two
// children: 0 and 1. The value and the children might be nil. An arc connects
// a node with its child. A node N has a value V and S is the bit string of
// the path from the root to N iff the trie maps S to V. Using this rule we can
// build maps and sets. For example, a set of {'b', 'c'} can be represented as:
// 'b': 0110 0010     o
// 'c': 0110 0011   0/
//                  1\
//                   1\
//                   0/
//                  0/
//                 0/
//                 1\
//                   o
//                 0/1\
//                 o   o
// This trie has 10 nodes. This representation is not efficient.
// To reduce the number of nodes, we use the path contraction technique.
//
// 2) Path contraction. If a path consists only of nodes that have one child
// and don't have a value, then the path can be replaced with one arc.
// The new arc has the whole bit string written on the path. The example above
// becomes smaller:            o
//                     0110001/
//                           o
//                         0/1\
//                         o   o
// This structure is stored in memory in a slightly different way. The trie consists
// of nodes, each node has a value and two children. Each non-nil child is
// a triple: the child node, the bit string written on the arc and the length of
// the bit string. For convenience, bits in the bit string are aligned so that
// the bit string might be a sub slice of a bit string representing a path from
// a root of the trie to the child.
//
// 3) Copy-on-write modifications. In order to support immutable views of
// the data in the ptrie, the Put() and the Delete() functions have the makeCopy
// flag. If the makeCopy flag is true, then the algorithm doesn't modify the
// current ptrie, but it returns a new ptrie with some nodes reused from
// the current one.
package ptrie

// T represents a ptrie.
type T struct {
	root        *pnode
	copyOnWrite bool
}

// New returns a new empty ptrie. If copyOnWrite is true, then the new ptrie
// performs copy-on-write modifications for Put/Delete operations and it is
// allowed to make a copy of the ptrie by calling Copy().
func New(copyOnWrite bool) *T {
	return &T{copyOnWrite: copyOnWrite}
}

// Put maps the given value to the given key. The value is not copied, so
// the client must keep the value unchanged.
func (t *T) Put(key []byte, value interface{}) {
	t.root = t.root.put(key, value, t.copyOnWrite)
}

// Get returns a value mapped to the given key. Get returns nil if the given
// key has no mapped value. The client must not modify the returned value as
// the returned value points directly to the value stored in the ptrie.
func (t *T) Get(key []byte) interface{} {
	return t.root.get(key)
}

// Delete removes mapping to the given key.
func (t *T) Delete(key []byte) {
	t.root = t.root.delete(key, t.copyOnWrite)
}

// Scan returns all key-value pairs with keys in range [start, limit).
// If limit is "", all key-value pairs with keys >= start are included.
func (t *T) Scan(start, limit []byte) *Stream {
	return t.root.Scan(start, limit)
}

// Copy returns a copy of the ptrie. This operation is only allowed if the ptrie
// was created with the copyOnWrite flag. Copy is a very fast operation since
// it just copies the pointer to the root of the ptrie.
// Panics if the ptrie was created with copyOnWrite=false.
func (t *T) Copy() *T {
	if !t.copyOnWrite {
		panic("the ptrie was not created in persistent mode")
	}
	return &T{
		root:        t.root,
		copyOnWrite: true,
	}
}

// pnode represents a node in the ptrie.
type pnode struct {
	value interface{}
	child [2]*pchild
}

// pchild represents a child of a node in a ptrie.
type pchild struct {
	node   *pnode
	bitstr []byte
	bitlen uint32
}

// put maps the given value to the given key, assuming the given node is
// the root of the ptrie. The value is not copied, so the client must keep
// the value unchanged. Put returns the new root of the ptrie.
// The client must not modify the returned value as the returned value points
// directly to the value stored in the ptrie.
//
// If the makeCopy flag is true, then the Put performs a copy-on-write
// modification.
//
// A nil node is treated as an empty ptrie.
func (node *pnode) put(key []byte, value interface{}, makeCopy bool) *pnode {
	if value == nil {
		return node.delete(key, makeCopy)
	}
	if node == nil {
		node = &pnode{}
	}
	return putInternal(node, 0, key, value, makeCopy)
}

// get returns a value mapped to the given key, assuming the given node is
// the root of the ptrie. Get returns nil if the given key has no mapped value.
//
// A nil node is treated as an empty ptrie.
func (node *pnode) get(key []byte) interface{} {
	if node == nil {
		return nil
	}
	return getInternal(node, 0, key)
}

// delete removes mapping to the given key, assuming the given node is
// the root of the ptrie. Delete returns the new root of the ptrie.
//
// If the makeCopy flag is true, then the Delete performs a copy-on-write
// modification.
//
// A nil node is treated as an empty ptrie.
func (node *pnode) delete(key []byte, makeCopy bool) *pnode {
	if node == nil {
		return nil
	}
	newNode, _ := deleteInternal(node, 0, key, makeCopy)
	if newNode.value == nil && newNode.child[0] == nil && newNode.child[1] == nil {
		return nil
	}
	return newNode
}

// putInternal does a DFS through the ptrie to find a node corresponding to
// the key and updates the value.
//
// Invariant: the first bitIndex bits of the key represent the path from
// the root to the current node.
func putInternal(node *pnode, bitIndex uint32, key []byte, value interface{}, makeCopy bool) *pnode {
	if makeCopy {
		node = copyNode(node)
	}
	if bitlen(key) == bitIndex {
		// The node corresponding to the key is found, update the value.
		node.value = value
		return node
	}
	// Pick the appropriate child and check that         o - node
	// the bit string of the path to the child node       \
	// matches the corresponding substring of the key.     ?
	// If not, then we need to insert a node              / \
	// in the middle of the path to the child.           ?   o - child.node
	currBit := getBit(key, bitIndex)
	if makeCopy {
		node.child[currBit] = copyChild(node.child[currBit])
	}
	child := node.child[currBit]
	lcp := bitLCP(child, key[bitIndex>>3:], bitIndex&7)
	if child != nil && lcp == child.bitlen {
		// child.bitstr matches the substring of the key.
		// Continue the DFS.
		child.node = putInternal(child.node, bitIndex+lcp, key, value, makeCopy)
		return node
	}
	// child.bitstr doesn't match the substring of the key.
	// We need to insert a node in the middle of the path to the child.
	//       o - node
	//        \A
	//         o - middleNode
	//        / \B
	//      C/   o - child.node
	//      o - newChild.node
	newChild := &pchild{
		node:   &pnode{value: value},
		bitstr: copyBytes(key[(bitIndex+lcp)>>3:]),
		bitlen: bitlen(key) - bitIndex - lcp,
	}
	if child == nil {
		// This case means that paths A and B are empty. Just attach the
		// new child to the node.
		node.child[currBit] = newChild
		return node
	}
	// Since the child.node exists and we picked the child based on the currBit
	// (a bit from the key), the path A is not empty.
	// The path B also can't be empty since lcp < child.bitlen.
	middleNode := new(pnode)
	// Update the child of the node, i.e. the A part.
	node.child[currBit] = &pchild{
		node:   middleNode,
		bitstr: child.bitstr[:((bitIndex&7)+lcp+7)>>3],
		bitlen: lcp,
	}
	// Pick the first bit on path C. Since C can be empty, we pick the first
	// bit on B and invert it.
	nextBit := getBit(child.bitstr, (bitIndex&7)+lcp) ^ 1
	// Set the C part only if C is not empty.
	if bitIndex+lcp < bitlen(key) {
		middleNode.child[nextBit] = newChild
	} else {
		middleNode.value = value
	}
	// Set the B part.
	middleNode.child[nextBit^1] = &pchild{
		node:   child.node,
		bitstr: child.bitstr[((bitIndex&7)+lcp)>>3:],
		bitlen: child.bitlen - lcp,
	}
	return node
}

// getInternal does a DFS through the ptrie to find a node corresponding to
// the key and returns the value.
//
// Invariant: the first bitIndex bits of the key represent the path from
// the root to the current node.
func getInternal(node *pnode, bitIndex uint32, key []byte) interface{} {
	keybitlen := bitlen(key)
	for keybitlen > bitIndex {
		child := node.child[getBit(key, bitIndex)]
		lcp := bitLCP(child, key[bitIndex>>3:], bitIndex&7)
		if child == nil || lcp != child.bitlen {
			return nil
		}
		bitIndex += lcp
		node = child.node
	}
	return node.value
}

// deleteInternal does a DFS through the ptrie to find a node corresponding to
// the key and deletes the value. deleteInternal removes the whole subtree if
// no nodes in the subtree have values.
//
// Invariant: the first bitIndex bits of the key represent the path from
// the root to the current node.
func deleteInternal(node *pnode, bitIndex uint32, key []byte, makeCopy bool) (newNode *pnode, deleted bool) {
	if bitlen(key) == bitIndex {
		// The node corresponding to the key is found.
		if node.child[0] == nil && node.child[1] == nil {
			return nil, true
		}
		if makeCopy {
			node = copyNode(node)
		}
		node.value = nil
		return node, true
	}
	// Pick the appropriate child and check that
	// the bit string of the path to the child node
	// matches the corresponding substring of the key.
	currBit := getBit(key, bitIndex)
	child := node.child[currBit]
	lcp := bitLCP(child, key[bitIndex>>3:], bitIndex&7)
	if child == nil || lcp != child.bitlen {
		// child.bitstr doesn't match the substring of the key, so the key
		// was not found in the ptrie.
		return node, false
	}
	// Delete the key in the subtree.
	if newNode, deleted = deleteInternal(child.node, bitIndex+lcp, key, makeCopy); !deleted {
		return node, false
	}
	if makeCopy {
		node = copyNode(node)
	}
	if newNode == nil {
		// If the whole subtree was removed, just remove the child.
		// It is possible that the node has no value and only one child. In this
		// case the node will be contracted one step out of the recursion.
		node.child[currBit] = nil
		return node, true
	}
	if makeCopy {
		node.child[currBit] = copyChild(node.child[currBit])
	}
	if newNode.value == nil && (newNode.child[0] == nil || newNode.child[1] == nil) {
		// Contract the new node if necessary.
		// Note: both children of the new node can't be nil since deleteInternal
		// automatically removes empty subtrees.
		child := newNode.child[0]
		if child == nil {
			child = newNode.child[1]
		}
		node.child[currBit].node = child.node
		node.child[currBit].bitstr = copyBytes(node.child[currBit].bitstr)
		node.child[currBit].bitstr = appendBits(node.child[currBit].bitstr, (bitIndex&7)+node.child[currBit].bitlen, child.bitstr)
		node.child[currBit].bitlen += child.bitlen
	} else {
		node.child[currBit].node = newNode
	}
	return node, true
}
