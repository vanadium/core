// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ptrie

import (
	"bytes"
	"math/rand"
	"reflect"
	"runtime/debug"
	"sort"
	"testing"
)

func deepCopy(node *pnode) *pnode {
	if node == nil {
		return nil
	}
	result := copyNode(node)
	for i := 0; i < 2; i++ {
		if result.child[i] != nil {
			result.child[i] = copyChild(result.child[i])
			result.child[i].node = deepCopy(result.child[i].node)
			result.child[i].bitstr = copyBytes(result.child[i].bitstr)
		}
	}
	return result
}

// TestPutGetDelete verifies basic functionality of Put/Get/Delete/Scan.
func TestPutGetDelete(t *testing.T) {
	data := New(true)
	data.Put([]byte("a"), "a")
	data.Put([]byte("ab"), "ab")
	data.Put([]byte("aaa"), nil)
	s := data.Scan([]byte("a"), []byte("ab"))
	if !s.Advance() {
		t.Fatalf("the stream didn't advance")
	}
	if got, want := s.Key(nil), []byte("a"); !bytes.Equal(got, want) {
		Fatalf(t, "unexpected key: got %q, want %q")
	}
	if got, want := s.Value().(string), "a"; got != want {
		Fatalf(t, "unexpected value: got %q, want %q")
	}
	for i := 0; i < 2; i++ {
		if s.Advance() {
			Fatalf(t, "the stream advanced")
		}
	}
	if got, want := data.Get([]byte("a")).(string), "a"; got != want {
		t.Fatalf("unexpected Get result: got %q, want %q", got, want)
	}
	if got, want := data.Get([]byte("ab")).(string), "ab"; got != want {
		t.Fatalf("unexpected Get result: got %q, want %q", got, want)
	}
	// Verify that copy-on-write works.
	newData := data.Copy()
	newData.Delete([]byte("a"))
	if got, want := data.Get([]byte("a")).(string), "a"; got != want {
		t.Fatalf("unexpected Get result: got %q, want %q", got, want)
	}
	if value := newData.Get([]byte("a")); value != nil {
		t.Fatalf("Get returned a non-nil value %v", value)
	}
	// Verify path contraction after Delete().
	if newData.root.child[0].bitlen != 16 {
		t.Fatal("path was not contracted after Delete()")
	}
	// Verify path contraction after Put("ac") and Delete("ac").
	data = newData.Copy()
	data.Put([]byte("ac"), "ac")
	if got, want := data.Get([]byte("ac")).(string), "ac"; got != want {
		t.Fatalf("unexpected Get result: got %q, want %q", got, want)
	}
	data.Delete([]byte("ab"))
	if data.root.child[0].bitlen != 16 {
		t.Fatal("path was not contracted after Delete()")
	}
}

// TestEmptyPtrie tests behavior of an empty ptrie.
func TestEmptyPtrie(t *testing.T) {
	data := New(false)
	if data.Get([]byte("abc")) != nil {
		t.Fatalf("Get() returned non-nil value")
	}
	data.Put([]byte("abc"), "abc")
	for i := 0; i < 2; i++ {
		data.Delete([]byte("abc"))
		if data.root != nil {
			t.Fatalf("non-nil root for an empty ptrie")
		}
	}
	s := data.Scan(nil, nil)
	for i := 0; i < 2; i++ {
		if s.Advance() {
			t.Fatalf("an empty stream advanced")
		}
	}
	s = data.Scan([]byte("abc"), nil)
	for i := 0; i < 2; i++ {
		if s.Advance() {
			t.Fatalf("an empty stream advanced")
		}
	}
}

// TestDeepPtrie verifies functionality of Put/Get/Delete/Scan on a trie with
// a long path.
func TestLongPath(t *testing.T) {
	r := rand.New(rand.NewSource(seed))
	depth := 16
	prefix := make([]byte, depth)
	for i := 0; i < depth; i++ {
		prefix[i] = byte(r.Intn(256))
	}
	var keys1, keys2 []string
	for i := 0; i < depth; i++ {
		keys1 = append(keys1, string(prefix[:i+1]))
		for j := 0; j < 8; j++ {
			key := copyBytes(prefix[:i+1])
			key[i] ^= 1 << uint32(j)
			keys2 = append(keys2, string(key))
		}
	}
	runPutGetDeleteTest(t, keys1)
	runScanTest(t, keys1)
	runPutGetDeleteTest(t, keys2)
	runScanTest(t, keys2)
	allKeys := append(keys2, keys1...)
	runPutGetDeleteTest(t, allKeys)
	runScanTest(t, allKeys)
}

// runPutGetDeleteTest adds the keys in random order checking the Put/Get/Delete
// behavior and verifying that copy-on-write mode doesn't modify previous
// versions.
func runPutGetDeleteTest(t *testing.T, keys []string) {
	noCopyOnWrite := New(false)
	copyOnWrite := New(true)
	r := rand.New(rand.NewSource(seed))
	perm := r.Perm(len(keys))
	// Add keys.
	for i := 0; i < len(keys); i++ {
		key := []byte(keys[perm[i]])
		oldVersion := copyOnWrite.Copy().root
		oldVersionCopy := deepCopy(oldVersion)
		copyOnWrite.Put(key, key)
		if !reflect.DeepEqual(oldVersion, oldVersionCopy) {
			Fatalf(t, "old version is modified after adding key %v", key)
		}
		noCopyOnWrite.Put(key, key)
		if !reflect.DeepEqual(noCopyOnWrite.root, copyOnWrite.root) {
			Fatalf(t, "ptrie with copyOnWrite and without are different after adding key %v", key)
		}
		for j := 0; j <= i; j++ {
			key := []byte(keys[perm[j]])
			if got, want := copyOnWrite.Get(key).([]byte), key; !bytes.Equal(got, want) {
				Fatalf(t, "unexpected value: got %v, want %v", got, want)
			}
		}
	}
	perm = r.Perm(len(keys))
	// Now delete keys.
	for i := len(keys) - 1; i >= 0; i-- {
		key := []byte(keys[perm[i]])
		oldVersion := copyOnWrite.Copy().root
		oldVersionCopy := deepCopy(oldVersion)
		copyOnWrite.Delete(key)
		if !reflect.DeepEqual(oldVersion, oldVersionCopy) {
			Fatalf(t, "old version is modified after adding key %v", key)
		}
		noCopyOnWrite.Delete(key)
		if !reflect.DeepEqual(noCopyOnWrite.root, copyOnWrite.root) {
			Fatalf(t, "ptrie with copyOnWrite and without are different after adding key %v", key)
		}
		for j := 0; j < i; j++ {
			key := []byte(keys[perm[j]])
			if got, want := copyOnWrite.Get(key).([]byte), key; !bytes.Equal(got, want) {
				Fatalf(t, "unexpected value: got %v, want %v", got, want)
			}
		}
	}
	if copyOnWrite.root != nil {
		t.Fatalf("non-nil root for an empty ptrie")
	}
	if noCopyOnWrite.root != nil {
		t.Fatalf("non-nil root for an empty ptrie")
	}
}

// runScanTest adds a random half of the keys and verifies streams started from
// every key in the keys slice.
func runScanTest(t *testing.T, keys []string) {
	sort.Strings(keys)
	r := rand.New(rand.NewSource(seed))
	perm := r.Perm(len(keys))
	used := make([]bool, len(keys))
	trie := New(false)
	for i := 0; i*2 < len(keys); i++ {
		j := perm[i]
		used[j] = true
		key := []byte(keys[j])
		trie.Put(key, key)
	}
	for l := 0; l < len(keys); l++ {
		s := trie.Scan([]byte(keys[l]), nil)
		for cur := l; cur < len(keys); cur++ {
			if !used[cur] {
				continue
			}
			if !s.Advance() {
				Fatalf(t, "the stream didn't advance")
			}
			if got, want := s.Key(nil), []byte(keys[cur]); !bytes.Equal(got, want) {
				Fatalf(t, "unexpected key: got %v, want %v")
			}
			if got, want := s.Value().([]byte), []byte(keys[cur]); !bytes.Equal(got, want) {
				Fatalf(t, "unexpected value: got %v, want %v")
			}
		}
		if s.Advance() {
			Fatalf(t, "the stream advanced")
		}
	}
}

func Fatalf(t *testing.T, format string, args ...interface{}) {
	debug.PrintStack()
	t.Fatalf(format, args...)
}
