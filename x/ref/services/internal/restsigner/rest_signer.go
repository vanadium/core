// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package restsigner

import (
	gocontext "context"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"math/big"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"v.io/v23/security"
	signer "v.io/x/ref/services/internal/restsigner/signer/v1"
)

func DecodePublicKey(k *signer.PublicKey) (*ecdsa.PublicKey, error) {
	bytes, err := base64.URLEncoding.DecodeString(k.Base64)
	if err != nil {
		return nil, err
	}
	key, err := x509.ParsePKIXPublicKey(bytes)
	if err != nil {
		return nil, err
	}
	pub, ok := key.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("Not an ECDSA public key")
	}
	return pub, nil
}

func DecodeSignature(sig *signer.VSignature) (r, s *big.Int, err error) {
	r, s = new(big.Int), new(big.Int)
	if _, ok := r.SetString(sig.R, 0); !ok {
		return nil, nil, fmt.Errorf("unable to parse big.Int %s", sig.R)
	}
	if _, ok := s.SetString(sig.S, 0); !ok {
		return nil, nil, fmt.Errorf("unable to parse big.Int %s", sig.S)
	}
	return
}

func NewRestSigner() (security.Signer, error) {
	client, err := signer.New(oauth2.NewClient(gocontext.TODO(), google.ComputeTokenSource("")))
	if err != nil {
		return nil, err
	}
	jkey, err := client.PublicKey().Do()
	if err != nil {
		return nil, err
	}
	key, err := DecodePublicKey(jkey)
	if err != nil {
		return nil, err
	}
	sign := func(message []byte) (r, s *big.Int, err error) {
		msgBase64 := base64.URLEncoding.EncodeToString(message)
		jsig, err := client.Sign(msgBase64).Do()
		if err != nil {
			return nil, nil, err
		}
		return DecodeSignature(jsig)
	}
	return security.NewECDSASigner(key, sign), nil
}
