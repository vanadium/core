// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build java android

package security

import (
	"log"
	"runtime"

	"v.io/v23/security"
	jutil "v.io/x/jni/util"
)

// #include "jni.h"
import "C"

// GoSigner creates an instance of security.Signer that uses the provided
// Java VSigner as its underlying implementation.
func GoSigner(env jutil.Env, jSigner jutil.Object) (security.Signer, error) {
	// Reference Java VSigner; it will be de-referenced when the Go Signer
	// created below is garbage-collected (through the finalizer callback we
	// setup just below).
	jSigner = jutil.NewGlobalRef(env, jSigner)
	s := &signer{
		jSigner: jSigner,
	}
	runtime.SetFinalizer(s, func(s *signer) {
		env, freeFunc := jutil.GetEnv()
		defer freeFunc()
		jutil.DeleteGlobalRef(env, s.jSigner)
	})
	return s, nil
}

type signer struct {
	jSigner jutil.Object
}

func (s *signer) Sign(purpose, message []byte) (security.Signature, error) {
	env, freeFunc := jutil.GetEnv()
	defer freeFunc()
	signatureSign := jutil.ClassSign("io.v.v23.security.VSignature")
	jSig, err := jutil.CallObjectMethod(env, s.jSigner, "sign", []jutil.Sign{jutil.ArraySign(jutil.ByteSign), jutil.ArraySign(jutil.ByteSign)}, signatureSign, purpose, message)
	if err != nil {
		return security.Signature{}, err
	}
	return GoSignature(env, jSig)
}

func (s *signer) PublicKey() security.PublicKey {
	env, freeFunc := jutil.GetEnv()
	defer freeFunc()
	publicKeySign := jutil.ClassSign("java.security.interfaces.ECPublicKey")
	jPublicKey, err := jutil.CallObjectMethod(env, s.jSigner, "publicKey", nil, publicKeySign)
	if err != nil {
		log.Printf("Couldn't get Java public key: %v", err)
		return nil
	}
	// Get the encoded version of the public key.
	encoded, err := jutil.CallByteArrayMethod(env, jPublicKey, "getEncoded", nil)
	if err != nil {
		log.Printf("Couldn't get encoded data for Java public key: %v", err)
		return nil
	}
	key, err := security.UnmarshalPublicKey(encoded)
	if err != nil {
		log.Printf("Couldn't parse Java ECDSA public key: " + err.Error())
		return nil
	}
	return key
}
