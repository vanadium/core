// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package transactions

import (
	"fmt"
)

// trie is an in-memory data structure that keeps track of recently-written
// keys, and exposes an interface for asking when a key or key range was most
// recently written to. It is used to check whether the read set of a
// transaction pending commit is still valid. The transaction can be committed
// iff its read set is valid.
// TODO(rogulenko): replace this dummy implementation with an actual trie.
type trie struct {
	seqs map[string]uint64
}

func newTrie() *trie {
	return &trie{
		seqs: make(map[string]uint64),
	}
}

// add updates the given key to the given seq, which must be greater than the
// current seq (if one exists). Seqs of subsequent calls must be in
// ascending order.
func (t *trie) add(key []byte, seq uint64) {
	keystr := string(key)
	if oldSeq, ok := t.seqs[keystr]; ok && seq < oldSeq {
		panic(fmt.Sprintf("seq for key %q should be at least %d, but got %d", key, oldSeq, seq))
	}
	t.seqs[keystr] = seq
}

// remove reverts effect of add(key, seq).
// Seqs of subsequent calls must be in ascending order.
func (t *trie) remove(key []byte, seq uint64) {
	keystr := string(key)
	oldSeq, ok := t.seqs[keystr]
	if !ok {
		panic(fmt.Sprintf("key %q was not found", key))
	}
	if oldSeq > seq {
		return
	} else if oldSeq == seq {
		delete(t.seqs, keystr)
	} else {
		panic(fmt.Sprintf("seq for key %q is too big: got %v, want %v", keystr, seq, oldSeq))
	}
}

// get returns the seq associated with the given key.
func (t *trie) get(key []byte) uint64 {
	keystr := string(key)
	if seq, ok := t.seqs[keystr]; ok {
		return seq
	}
	return 0
}

// rangeMax returns the max seq associated with keys in range
// [start, limit). Empty limit means no limit.
func (t *trie) rangeMax(start, limit []byte) uint64 {
	var result uint64 = 0
	s, e := string(start), string(limit)
	for key, seq := range t.seqs {
		if key >= s && (e == "" || key < e) && seq > result {
			result = seq
		}
	}
	return result
}
