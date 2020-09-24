// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conn

import (
	"crypto/sha256"
	"sync"

	"v.io/v23/context"
	"v.io/v23/security"
	"v.io/v23/vom"
	"v.io/x/ref/lib/security/bcrypter"
)

var encryptionCache = encCache{m: make(map[encCacheKey]bcrypter.WireCiphertext)}

// Maximum no of elements in the cache is 64. This makes the size of
// the cache = (160 + 16 + 32)*64 + (total size of the 64 plaintexts).
// Assume that the average plaintext size is upperbounded by 1KB,
// the size of the cache is upperbound by 77KB.
const encCacheMaxSize = 1 << 6

type encCacheKey struct {
	pattern       security.BlessingPattern
	plaintextHash [sha256.Size]byte
}

type encCache struct {
	sync.RWMutex
	m map[encCacheKey]bcrypter.WireCiphertext
}

func (c *encCache) evictIfNeededLocked() {
	// Randomly evict an entry. Fortunately, map iteration is in random key order
	// (see "Iteration Order" in http://blog.golang.org/go-maps-in-action)
	toEvict := len(c.m) - encCacheMaxSize
	if toEvict <= 0 {
		return
	}
	n := 0
	for key := range c.m {
		delete(c.m, key)
		n++
		if n >= toEvict {
			break
		}
	}
}

func (c *encCache) cache(pattern security.BlessingPattern, plaintext []byte, ctxt bcrypter.WireCiphertext) {
	key := encCacheKey{pattern: pattern, plaintextHash: sha256.Sum256(plaintext)}
	c.Lock()
	defer c.Unlock()
	c.m[key] = ctxt
	c.evictIfNeededLocked()
}

func (c *encCache) ciphertext(pattern security.BlessingPattern, plaintext []byte) (bcrypter.WireCiphertext, bool) {
	key := encCacheKey{pattern: pattern, plaintextHash: sha256.Sum256(plaintext)}
	c.RLock()
	defer c.RUnlock()
	ctxt, b := c.m[key]
	return ctxt, b
}

func encrypt(ctx *context.T, patterns []security.BlessingPattern, v interface{}) ([]bcrypter.WireCiphertext, error) {
	crypter := bcrypter.GetCrypter(ctx)
	if crypter == nil {
		return nil, ErrNoCrypter.Errorf(ctx, "no blessings-based crypter available")
	}
	b, err := vom.Encode(v)
	if err != nil {
		return nil, err
	}
	ciphertexts := make([]bcrypter.WireCiphertext, len(patterns))
	for i, p := range patterns {
		if ctxt, exists := encryptionCache.ciphertext(p, b); exists {
			ciphertexts[i] = ctxt
			continue
		}
		ctxt, err := crypter.Encrypt(ctx, p, b)
		if err != nil {
			return nil, err
		}
		ctxt.ToWire(&ciphertexts[i])
		encryptionCache.cache(p, b, ciphertexts[i])
	}
	return ciphertexts, nil
}

func decrypt(ctx *context.T, ciphertexts []bcrypter.WireCiphertext, v interface{}) error {
	crypter := bcrypter.GetCrypter(ctx)
	if crypter == nil {
		return ErrNoCrypter.Errorf(ctx, "no blessings-based crypter available")
	}
	var ctxt bcrypter.Ciphertext

	for _, c := range ciphertexts {
		ctxt.FromWire(c)
		if data, err := crypter.Decrypt(ctx, &ctxt); err != nil {
			continue
		} else if err := vom.Decode(data, v); err != nil {
			// Since we use a CCA-2 secure IBE scheme, the ciphertext
			// is not malleable. Therefore if decryption succeeds it
			// ought to be that this crypter has the appropriate private
			// key. Any errors in vom decoding the decrypted plaintext
			// are system errors and must be returned.
			return err
		} else {
			return nil
		}
	}
	return ErrNoPrivateKey.Errorf(ctx, "no blessings private key available for decryption")
}
