// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"sync"
	"syscall"
	"time"

	"v.io/v23/security"
	"v.io/x/lib/vlog"
	"v.io/x/ref/internal/logger"
	"v.io/x/ref/lib/security/internal/lockedfile"
	"v.io/x/ref/lib/security/serialization"
)

// blessingRoots implements security.BlessingRoots.
type blessingRoots struct {
	readers   SerializerReader
	writers   SerializerWriter
	signer    serialization.Signer
	publicKey security.PublicKey
	flock     *lockedfile.Mutex // GUARDS persistent store
	mu        sync.RWMutex
	state     blessingRootsState // GUARDED_BY(mu)
}

func (br *blessingRoots) Add(root []byte, pattern security.BlessingPattern) error {
	if pattern == security.AllPrincipals {
		return fmt.Errorf("a root cannot be recognized for all blessing names (i.e., the pattern '...')")
	}
	// Sanity check to avoid invalid keys being added.
	if _, err := security.UnmarshalPublicKey(root); err != nil {
		return err
	}
	key := string(root)
	br.mu.Lock()
	defer br.mu.Unlock()

	unlock, err := br.writeLockAndLoad()
	if err != nil {
		return err
	}
	defer unlock()

	patterns := br.state[key]
	for _, p := range patterns {
		if p == pattern {
			return nil
		}
	}
	br.state[key] = append(patterns, pattern)

	if err := br.save(); err != nil {
		br.state[key] = patterns[:len(patterns)-1]
		return err
	}

	return nil
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

type root struct {
	key      security.PublicKey
	patterns string
}

type rootSorter []*root

func (s rootSorter) Len() int           { return len(s) }
func (s rootSorter) Less(i, j int) bool { return s[i].patterns < s[j].patterns }
func (s rootSorter) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func (br *blessingRoots) save() error {
	if (br.signer == nil) && (br.writers == nil) {
		return nil
	}
	data, signature, err := br.writers.Writers()
	if err != nil {
		return err
	}
	return encodeAndStore(br.state, data, signature, br.signer)
}

func (br *blessingRoots) readLockAndLoad() (func(), error) {
	return readLockAndLoad(br.flock, br.load)
}

func (br *blessingRoots) writeLockAndLoad() (func(), error) {
	return writeLockAndLoad(br.flock, br.load)
}

func (br *blessingRoots) load() error {
	if br.readers == nil {
		return nil
	}
	data, signature, err := br.readers.Readers()
	if err != nil {
		return err
	}
	if data == nil && signature == nil {
		return nil
	}
	state := make(blessingRootsState)
	if err := decodeFromStorage(&state, data, signature, br.publicKey); err != nil {
		return fmt.Errorf("failed to load BlessingRoots: %v", err)
	}
	br.state = state
	return nil
}

func reload(ctx context.Context, loader func() (func(), error), hupCh <-chan os.Signal, update time.Duration) {
	for {
		select {
		case <-time.After(update):
		case <-hupCh:
		case <-ctx.Done():
			return
		}
		unlock, err := loader()
		if err != nil {
			logger.Global().Infof("failed top reload principal: %v", err)
			continue
		}
		unlock()
	}
}

// NewBlessingRoots returns an implementation of security.BlessingRoots
// that keeps all state in memory. The returned BlessingRoots is initialized
// with an empty set of keys.
func NewBlessingRoots() security.BlessingRoots {
	return &blessingRoots{
		state: make(blessingRootsState),
	}
}

// NewPersistentBlessingRoots returns a security.BlessingRoots for a principal
// that is initialized with the persisted data. The returned security.BlessingStore
// will persists any updates to its state if the supplied writers serializer
// is specified.
func NewPersistentBlessingRoots(ctx context.Context, lockFilePath string, readers SerializerReader, writers SerializerWriter, signer serialization.Signer, publicKey security.PublicKey, update time.Duration) (security.BlessingRoots, error) {
	if readers == nil || (writers != nil && signer == nil) {
		return nil, fmt.Errorf("blessing's public key does not match store's public key")
	}
	br := &blessingRoots{
		flock:   lockedfile.MutexAt(lockFilePath),
		readers: readers,
		writers: writers,
		signer:  signer,
		state:   make(blessingRootsState),
	}
	if signer != nil {
		br.publicKey = signer.PublicKey()
	} else {
		br.publicKey = publicKey
	}
	if err := br.load(); err != nil {
		return nil, err
	}
	if update > 0 {
		hupCh := make(chan os.Signal, 1)
		signal.Notify(hupCh, syscall.SIGHUP)
		go reload(ctx, func() (func(), error) {
			br.mu.Lock()
			defer br.mu.Unlock()
			return br.readLockAndLoad()
		}, hupCh, update)
	}
	return br, nil
}
