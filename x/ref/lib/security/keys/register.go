// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package keys

import (
	"context"
	"crypto"
	"encoding/pem"
	"fmt"
	"reflect"
	"sync"

	"v.io/v23/security"
)

// MarshalPublicKeyFunc marshals the supplied key to PEM.
type MarshalPublicKeyFunc func(crypto.PublicKey) ([]byte, error)

// MarshalPrivateKeyFunc marshals the supplied key to PEM optionally
// encrypting it using the supplied passphrase.
type MarshalPrivateKeyFunc func(key crypto.PrivateKey, passphrase []byte) ([]byte, error)

// ParsePublicKeyFunc parses a public key from PEM.
type ParsePublicKeyFunc func(block *pem.Block) (crypto.PublicKey, error)

// ParsePublicKeyTextFunc parses a public key from a format other than PEM.
type ParsePublicKeyTextFunc func(data []byte) (crypto.PublicKey, error)

// ParsePrivateKeyFunc parses a private key from PEM.
type ParsePrivateKeyFunc func(block *pem.Block) (crypto.PrivateKey, error)

// DecryptFunc decrypts a pem block using the supplied passphrase.
type DecryptFunc func(block *pem.Block, passphrase []byte) (*pem.Block, crypto.PrivateKey, error)

// API represents a common set of operations that can be implemented for
// specific key types.
type API interface {
	Signer(ctx context.Context, key crypto.PrivateKey) (security.Signer, error)
	//SignerOpts(key crypto.PrivateKey, opts ...interface{}) (security.Signer, error)
	PublicKey(key interface{}) (security.PublicKey, error)
}

type Registrar struct {
	mu              sync.RWMutex
	marshalPrivate  map[string]MarshalPrivateKeyFunc
	marshalPublic   map[string]MarshalPublicKeyFunc
	apis            map[string]API
	parsePrivate    map[string][]parserInfo
	parsePublic     map[string][]parserInfo
	parsePublicText []ParsePublicKeyTextFunc
	decrypters      map[string][]parserInfo
}

func NewRegistrar() *Registrar {
	return &Registrar{
		marshalPrivate:  map[string]MarshalPrivateKeyFunc{},
		marshalPublic:   map[string]MarshalPublicKeyFunc{},
		apis:            map[string]API{},
		parsePrivate:    map[string][]parserInfo{},
		parsePublic:     map[string][]parserInfo{},
		parsePublicText: []ParsePublicKeyTextFunc{},
		decrypters:      map[string][]parserInfo{},
	}
}

func (r *Registrar) RegisterPublicKeyMarshaler(fn MarshalPublicKeyFunc, types ...interface{}) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, typ := range types {
		r.marshalPublic[reflect.TypeOf(typ).String()] = fn
	}
}

func (r *Registrar) RegisterPrivateKeyMarshaler(fn MarshalPrivateKeyFunc, types ...interface{}) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, typ := range types {
		r.marshalPrivate[reflect.TypeOf(typ).String()] = fn
	}
}

func (r *Registrar) RegisterAPI(ifc API, types ...interface{}) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, typ := range types {
		typ := reflect.TypeOf(typ).String()
		if _, ok := r.apis[typ]; ok {
			return fmt.Errorf("type %v already registered", typ)
		}
		r.apis[typ] = ifc
	}
	return nil
}

type parserInfo struct {
	public    ParsePublicKeyFunc
	private   ParsePrivateKeyFunc
	decrypter DecryptFunc
	matcher   func(*pem.Block) bool
}

func matchAny(*pem.Block) bool {
	return true
}

func (r *Registrar) RegisterPublicKeyParser(parser ParsePublicKeyFunc, pemType string, matcher func(*pem.Block) bool) {
	if matcher == nil {
		matcher = matchAny
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.parsePublic[pemType] = append(r.parsePublic[pemType], parserInfo{public: parser, matcher: matcher})

}

func (r *Registrar) RegisterPrivateKeyParser(parser ParsePrivateKeyFunc, pemType string, matcher func(*pem.Block) bool) {
	if matcher == nil {
		matcher = matchAny
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.parsePrivate[pemType] = append(r.parsePrivate[pemType], parserInfo{private: parser, matcher: matcher})
}

func (r *Registrar) RegisterDecrypter(decrypter DecryptFunc, pemType string, matcher func(*pem.Block) bool) {
	if matcher == nil {
		matcher = matchAny
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.decrypters[pemType] = append(r.decrypters[pemType], parserInfo{decrypter: decrypter, matcher: matcher})
}

func (r *Registrar) RegisterPublicKeyTextParser(parser ParsePublicKeyTextFunc) {
	r.mu.Lock()
	r.parsePublicText = append(r.parsePublicText, parser)
	r.mu.Unlock()
}

func (r *Registrar) RegisterIndirectPrivateKeyParser(parser ParsePrivateKeyFunc, typ string) {
	r.RegisterPrivateKeyParser(parser,
		IndirectionPrivateKeyPEMType,
		IndirectMatcherFunc(typ))
}

func (r *Registrar) getMarshallers(typ interface{}) (MarshalPublicKeyFunc, MarshalPrivateKeyFunc) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	k := reflect.TypeOf(typ).String()
	return r.marshalPublic[k], r.marshalPrivate[k]
}

func (r *Registrar) MarshalPublicKey(key crypto.PublicKey) ([]byte, error) {
	pub, _ := r.getMarshallers(key)
	if pub == nil {
		return nil, fmt.Errorf("MarshalPublicKey: unsupported type %T", key)
	}
	return pub(key)
}

func (r *Registrar) MarshalPrivateKey(key crypto.PrivateKey, passphrase []byte) ([]byte, error) {
	_, priv := r.getMarshallers(key)
	if priv == nil {
		return nil, fmt.Errorf("MarshalPrivateKey: unsupported type %T", key)
	}
	return priv(key, passphrase)
}

func (r *Registrar) ParsePublicKey(data []byte) (crypto.PublicKey, error) {
	key, err := r.parsePublicKeys(data)
	if err == nil {
		return key, nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, fn := range r.parsePublicText {
		if key, err := fn(data); err == nil {
			return key, nil
		}
	}
	return nil, err
}

func (r *Registrar) ParsePrivateKey(ctx context.Context, data, passphrase []byte) (crypto.PrivateKey, error) {
	return r.parsePrivateKeys(ctx, data, passphrase, true)
}

func (r *Registrar) APIForKey(key crypto.PrivateKey) (API, error) {
	r.mu.RLock()
	api, ok := r.apis[reflect.TypeOf(key).String()]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("APIForKey: unsupported type %T", key)
	}
	return api, nil
}

func (r *Registrar) getParserForBlock(funcMap map[string][]parserInfo, block *pem.Block) (parserInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	pis, ok := funcMap[block.Type]
	if !ok {
		return parserInfo{}, false
	}
	for _, pi := range pis {
		if pi.matcher == nil {
			continue
		}
		if pi.matcher(block) {
			return pi, true
		}
	}
	for _, pi := range pis {
		if pi.matcher == nil {
			return pi, true
		}
	}
	return parserInfo{}, false
}

func (r *Registrar) parsePublicKeys(pemBlockBytes []byte) (crypto.PrivateKey, error) {
	var pemBlock *pem.Block
	for {
		pemBlock, pemBlockBytes = pem.Decode(pemBlockBytes)
		if pemBlock == nil {
			return nil, fmt.Errorf("processed all PEM blocks without finding a private key")
		}
		parser, ok := r.getParserForBlock(r.parsePublic, pemBlock)
		if !ok {
			continue
		}
		return parser.public(pemBlock)
	}
}

func (r *Registrar) parsePrivateKeys(ctx context.Context, pemBlockBytes, passphrase []byte, followLinks bool) (crypto.PrivateKey, error) {
	for {
		var pemBlock *pem.Block
		pemBlock, pemBlockBytes = pem.Decode(pemBlockBytes)
		if pemBlock == nil {
			return nil, fmt.Errorf("processed all PEM blocks without finding a private key")
		}
		decrypt, ok := r.getParserForBlock(r.decrypters, pemBlock)
		var plainText *pem.Block
		if ok {
			var plainKey crypto.PrivateKey
			var err error
			plainText, plainKey, err = decrypt.decrypter(pemBlock, passphrase)
			if err == nil && plainKey != nil {
				return plainKey, nil
			}
			if err != nil {
				return nil, err
			}
		}
		parser, ok := r.getParserForBlock(r.parsePrivate, pemBlock)
		if !ok {
			continue
		}
		if plainText == nil {
			plainText = pemBlock
		}
		key, err := parser.private(plainText)
		if err != nil {
			return nil, err
		}
		// Follow at most one link.
		if indirection, ok := key.(IndirectPrivateKey); ok {
			if !followLinks {
				return nil, fmt.Errorf("indirection limit reached for %v", indirection)
			}
			key, data := indirection.Next(ctx)
			if key != nil {
				return key, nil
			}
			return r.parsePrivateKeys(ctx, data, passphrase, false)
		}
		return key, nil
	}
}
