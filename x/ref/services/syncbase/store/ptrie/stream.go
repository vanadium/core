// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ptrie

import (
	"bytes"

	"v.io/x/ref/services/syncbase/store"
)

// Stream is a struct for iterating through a ptrie.
// WARNING: The stream is not thread-safe.
//
// The best way to walk through nodes of a tree is to perform a DFS.
// To support the Advance() method, we save the current state of the DFS
// by capturing the DFS stack.
type Stream struct {
	dfsStack    []*dfsStackElement
	limit       []byte
	hasAdvanced bool // true iff the stream has been advanced at least once
	hasValue    bool // true iff a value has been staged
	key         []byte
}

type dfsStackElement struct {
	node    *pnode
	childID int8
}

// Scan returns all key-value pairs with keys in range [start, limit).
// If limit is "", all key-value pairs with keys >= start are included.
//
// A nil node is treated as an empty ptrie.
func (node *pnode) Scan(start, limit []byte) *Stream {
	if node == nil {
		return &Stream{}
	}
	s := &Stream{
		limit: copyBytes(limit),
	}
	// Locate the first key-value pair with key >= start and capture
	// the DFS stack.
	s.lowerBound(node, 0, start)
	return s
}

// Advance stages an element so the client can retrieve it with Key or Value.
// Advance returns true iff there is an element to retrieve. The client must
// call Advance before calling Key or Value.
func (s *Stream) Advance() bool {
	s.hasValue = false
	if len(s.dfsStack) == 0 {
		return false
	}
	if !s.hasAdvanced {
		s.hasAdvanced = true
	} else {
		s.advanceInternal()
	}
	s.key = s.keyFromDFSStack()
	if len(s.dfsStack) == 0 || (len(s.limit) != 0 && bytes.Compare(s.key, s.limit) >= 0) {
		s.dfsStack = nil
		return false
	}
	s.hasValue = true
	return true
}

// Key returns the key of the element that was staged by Advance. The returned
// slice may be a sub-slice of keybuf if keybuf was large enough to hold the
// entire key. Otherwise, a newly allocated slice will be returned. It is
// valid to pass a nil keybuf.
// Key may panic if Advance returned false or was not called at all.
func (s *Stream) Key(keybuf []byte) []byte {
	if !s.hasValue {
		panic("nothing staged")
	}
	return store.CopyBytes(keybuf, s.key)
}

// Value returns the value of the element that was staged by Advance.
// The client should not modify the returned value as the returned
// value points directly to the value stored in the ptrie.
// Value may panic if Advance returned false or was not called at all.
func (s *Stream) Value() interface{} {
	if !s.hasValue {
		panic("nothing staged")
	}
	return s.dfsStack[len(s.dfsStack)-1].node.value
}

// advanceInternal simulates the DFS until the next node with a value is found
// or the stream reaches the end of the ptrie.
func (s *Stream) advanceInternal() {
	// The code below simulates the following recursive function:
	// func dfs(node *Node) {
	// 	if node.Value != nil {
	// 		// The next key-value pair is found.
	// 	}
	// 	for i := 0; i < 2; i++ {
	// 		if node.child[i] != nil {
	// 			dfs(node.child[i].node)
	// 		}
	// 	}
	// }
	for len(s.dfsStack) > 0 {
		top := s.dfsStack[len(s.dfsStack)-1]
		// childID can be -1, 0, or 1.
		top.childID++
		for top.childID <= 1 && top.node.child[top.childID] == nil {
			top.childID++
		}
		if top.childID > 1 {
			s.popFromDFSStack()
			continue
		}
		child := top.node.child[top.childID].node
		s.dfsStack = append(s.dfsStack, &dfsStackElement{
			node:    child,
			childID: -1,
		})
		if child.value != nil {
			return
		}
	}
}

// keyFromDFSStack returns the bit string written on the path from the root
// to the node on the top of the DFS stack.
func (s *Stream) keyFromDFSStack() []byte {
	// Calculate the buffer size.
	var bitlen uint32 = 0
	for i := 0; i < len(s.dfsStack)-1; i++ {
		element := s.dfsStack[i]
		bitlen += element.node.child[element.childID].bitlen
	}
	if bitlen%8 != 0 {
		panic("the number of bytes in a key is not an integer")
	}
	// Allocate the buffer.
	buf := make([]byte, bitlen>>3)
	bitlen = 0
	// Concatenate all key parts.
	for i := 0; i < len(s.dfsStack)-1; i++ {
		element := s.dfsStack[i]
		buf = appendBits(buf, bitlen, element.node.child[element.childID].bitstr)
		bitlen += element.node.child[element.childID].bitlen
	}
	return buf
}

// lowerBound initializes the DFS stack to store a path to the first node with
// a key >= start and a non-nil value. lowerBound returns true iff that node
// was found in the subtree.
//
// Invariant: the first bitIndex bits of start represent the path from
// the root to the current node.
func (s *Stream) lowerBound(node *pnode, bitIndex uint32, start []byte) bool {
	if bitIndex >= bitlen(start) {
		s.findFirst(node)
		return true
	}
	top := s.pushToDFSStack(node)
	// Pick the appropriate child and check that
	// the bit string of the path to the child node
	// matches the corresponding substring of start.
	currBit := getBit(start, bitIndex)
	top.childID = int8(currBit)
	if child := node.child[currBit]; child != nil {
		lcp := bitLCP(child, start[bitIndex>>3:], bitIndex&7)
		if lcp < child.bitlen {
			// child.bitstr doesn't match the substring of start.
			// Find the first node with a value in a subtree of the child if
			// child.bitstr is greater than the corresponding substring of
			// start.
			if bitIndex+lcp == bitlen(start) || (bitIndex+lcp < bitlen(start) && getBit(start, bitIndex+lcp) == 0) {
				s.findFirst(child.node)
				return true
			}
		} else if s.lowerBound(child.node, bitIndex+lcp, start) {
			return true
		}
	}
	if currBit == 0 && node.child[1] != nil {
		top.childID = 1
		s.findFirst(node.child[1].node)
		return true
	}
	s.popFromDFSStack()
	return false
}

// findFirst simulates the DFS to find the first node with a value in a subtree.
// NOTE: since the Put/Delete implementation automatically removes
// subtrees without values, each subtree of a ptrie has a value.
func (s *Stream) findFirst(node *pnode) {
	top := s.pushToDFSStack(node)
	if node.value != nil {
		return
	}
	for top.childID < 1 {
		top.childID++
		if child := node.child[top.childID]; child != nil {
			s.findFirst(child.node)
			return
		}
	}
	panic("subtree has no nodes with values")
}

func (s *Stream) pushToDFSStack(node *pnode) *dfsStackElement {
	top := &dfsStackElement{
		node:    node,
		childID: -1,
	}
	s.dfsStack = append(s.dfsStack, top)
	return top
}

func (s *Stream) popFromDFSStack() {
	s.dfsStack = s.dfsStack[:len(s.dfsStack)-1]
}
