// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"v.io/v23/security"
	"v.io/x/lib/vlog"
	"v.io/x/ref/lib/security/serialization"
)

type blessingRoots struct {
	ctx   context.Context
	mu    sync.RWMutex
	state blessingRootsState // GUARDED_BY(mu)
}

func (br *blessingRoots) addLocked(root []byte, pattern security.BlessingPattern) (func(), error) {
	if pattern == security.AllPrincipals {
		return nil, fmt.Errorf("a root cannot be recognized for all blessing names (i.e., the pattern '...')")
	}
	// Sanity check to avoid invalid keys being added.
	if _, err := security.UnmarshalPublicKey(root); err != nil {
		return nil, err
	}
	key := string(root)
	patterns := br.state[key]
	for _, p := range patterns {
		if p == pattern {
			return func() {}, nil
		}
	}
	oldpatterns := br.state[key]
	undo := func() {
		br.state[key] = oldpatterns
	}
	br.state[key] = append(patterns, pattern)
	return undo, nil
}

func (br *blessingRoots) Add(root []byte, pattern security.BlessingPattern) error {
	br.mu.Lock()
	defer br.mu.Unlock()
	_, err := br.addLocked(root, pattern)
	return err
}

func (br *blessingRoots) Recognized(root []byte, blessing string) error {
	br.mu.RLock()
	for _, p := range br.state[string(root)] {
		if p.MatchedBy(blessing) {
			br.mu.RUnlock()
			return nil
		}
	}
	br.mu.RUnlock()

	// Silly to have to unmarshal the public key on an error.
	// Change the error message to not require that?
	obj, err := security.UnmarshalPublicKey(root)
	if err != nil {
		return err
	}

	return security.ErrorfUnrecognizedRoot(nil, "unrecognized public key %v in root certificate: %v", obj.String(), nil)
}

func (br *blessingRoots) RecognizedCert(root *security.Certificate, blessing string) error {
	return br.Recognized(root.PublicKey, blessing)
}

func (br *blessingRoots) Dump() map[security.BlessingPattern][]security.PublicKey {
	dump := make(map[security.BlessingPattern][]security.PublicKey)
	br.mu.RLock()
	defer br.mu.RUnlock()
	for keyStr, patterns := range br.state {
		key, err := security.UnmarshalPublicKey([]byte(keyStr))
		if err != nil {
			vlog.Errorf("security.UnmarshalPublicKey(%v) returned error: %v", []byte(keyStr), err)
			return nil
		}
		for _, p := range patterns {
			dump[p] = append(dump[p], key)
		}
	}
	return dump
}

type root struct {
	key      security.PublicKey
	patterns string
}

type rootSorter []*root

func (s rootSorter) Len() int           { return len(s) }
func (s rootSorter) Less(i, j int) bool { return s[i].patterns < s[j].patterns }
func (s rootSorter) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

// DebugString return a human-readable string encoding of the roots
// DebugString encodes all roots into a string in the following
// format
//
// Public key     Pattern
// <public key>   <patterns>
// ...
// <public key>   <patterns>
func (br *blessingRoots) DebugString() string {
	const format = "%-47s   %s\n"
	b := bytes.NewBufferString(fmt.Sprintf(format, "Public key", "Pattern"))
	var s rootSorter
	br.mu.RLock()
	defer br.mu.RUnlock()
	for keyBytes, patterns := range br.state {
		key, err := security.UnmarshalPublicKey([]byte(keyBytes))
		if err != nil {
			return fmt.Sprintf("failed to decode public key: %v", err)
		}
		s = append(s, &root{key, fmt.Sprintf("%v", patterns)})
	}
	sort.Sort(s)
	for _, r := range s {
		b.WriteString(fmt.Sprintf(format, r.key, r.patterns))
	}
	return b.String()
}

// NewBlessingRoots returns an implementation of security.BlessingRoots
// that keeps all state in memory. The returned BlessingRoots is initialized
// with an empty set of keys.
func NewBlessingRoots() security.BlessingRoots {
	return &blessingRoots{ctx: context.TODO(), state: make(blessingRootsState)}
}

// NewBlessingRootsOpts returns an implementation of security.BlessingRoots
// according to the supplied options.
// If no options are supplied all state is kept in memory.
func NewBlessingRootsOpts(ctx context.Context, opts ...CredentialsStoreOption) (security.BlessingRoots, error) {
	var o credentialsStoreOption
	for _, fn := range opts {
		fn(&o)
	}
	if o.reader == nil && o.writer == nil {
		return &blessingRoots{ctx: ctx, state: make(blessingRootsState)}, nil
	}
	if o.writer != nil {
		return newWritableBlessingRoots(ctx, o)
	}
	return newReadonlyBlessingRoots(ctx, o)
}

type blessingRootsReader struct {
	blessingRoots
	publicKey security.PublicKey
	interval  time.Duration
}

func newBlessingRootsReader(ctx context.Context, interval time.Duration, key security.PublicKey) blessingRootsReader {
	return blessingRootsReader{
		blessingRoots: blessingRoots{ctx: ctx, state: make(blessingRootsState)},
		publicKey:     key,
	}
}

func (br *blessingRootsReader) loadLocked(ctx context.Context, reader CredentialsStoreReader, publicKey security.PublicKey) error {
	rd, err := reader.RootsReader(ctx)
	if err != nil {
		return err
	}
	data, signature, err := rd.Readers()
	if err != nil {
		return err
	}
	state := make(blessingRootsState)
	if err := decodeFromStorage(&state, data, signature, publicKey); err != nil {
		return fmt.Errorf("failed to load BlessingRoots: %v", err)
	}
	br.state = state
	return nil
}

func (br *blessingRootsReader) load(ctx context.Context, reader CredentialsStoreReader, publicKey security.PublicKey) error {
	br.mu.Lock()
	defer br.mu.Unlock()
	unlock, err := reader.RLock(ctx, LockBlessingRoots)
	if err != nil {
		return err
	}
	defer unlock()
	return br.loadLocked(ctx, reader, publicKey)
}

func (br *blessingRootsReader) refresh(ctx context.Context, store CredentialsStoreReader) error {
	if err := br.load(ctx, store, br.publicKey); err != nil {
		return err
	}
	if br.interval == 0 {
		return nil
	}
	handleRefresh(ctx, br.interval, func() error {
		return br.load(ctx, store, br.publicKey)
	})
	return nil
}

type blessingRootsWritable struct {
	blessingRootsReader
	store  CredentialsStoreReadWriter
	signer serialization.Signer
}

func (br *blessingRootsWritable) Add(root []byte, pattern security.BlessingPattern) error {
	br.mu.Lock()
	defer br.mu.Unlock()

	unlock, err := br.store.Lock(br.ctx, LockBlessingRoots)
	if err != nil {
		return err
	}
	defer unlock()

	if err := br.loadLocked(br.ctx, br.store, br.publicKey); err != nil {
		return err
	}
	undo, err := br.addLocked(root, pattern)
	if err != nil {
		return err
	}
	if err := saveLocked(br.ctx, br.state, br.store, br.signer); err != nil {
		undo()
		return err
	}
	return nil
}

func newWritableBlessingRoots(ctx context.Context, opts credentialsStoreOption) (security.BlessingRoots, error) {
	br := &blessingRootsWritable{
		blessingRootsReader: newBlessingRootsReader(ctx, opts.updateInterval, opts.publicKey),
		store:               opts.writer,
		signer:              opts.signer,
	}
	if err := br.refresh(ctx, opts.writer); err != nil {
		return nil, err
	}
	return br, nil
}

type blessingRootsReadonly struct {
	blessingRootsReader
	store     CredentialsStoreReader
	publicKey security.PublicKey
}

func (br *blessingRootsReadonly) Add(root []byte, pattern security.BlessingPattern) error {
	return fmt.Errorf("Add is not implemented for readonly blessings roots")
}

func newReadonlyBlessingRoots(ctx context.Context, opts credentialsStoreOption) (security.BlessingRoots, error) {
	br := &blessingRootsReadonly{
		blessingRootsReader: newBlessingRootsReader(ctx, opts.updateInterval, opts.publicKey),
		store:               opts.reader,
		publicKey:           opts.publicKey,
	}
	if err := br.refresh(ctx, opts.reader); err != nil {
		return nil, err
	}
	return br, nil
}
