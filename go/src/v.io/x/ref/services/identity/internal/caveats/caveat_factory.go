// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package caveats

import (
	"fmt"
	"time"

	"v.io/x/ref/services/identity/internal/revocation"

	"v.io/v23/security"
)

type CaveatFactory interface {
	New(caveatInfo CaveatInfo) (security.Caveat, error)
}

type CaveatInfo struct {
	Type string
	Args []interface{}
}

type caveatFactory map[string]func(args ...interface{}) (security.Caveat, error)

func NewCaveatFactory() CaveatFactory {
	return caveatFactory{
		"Expiry":        expiryCaveat,
		"Method":        methodCaveat,
		"PeerBlessings": peerBlessingsCaveat,
		"Revocation":    revocationCaveat,
	}
}

func (c caveatFactory) New(caveatInfo CaveatInfo) (security.Caveat, error) {
	fact, exists := c[caveatInfo.Type]
	if !exists {
		return security.Caveat{}, fmt.Errorf("caveat %s does not exist in CaveatFactory", caveatInfo.Type)
	}
	return fact(caveatInfo.Args...)
}

func expiryCaveat(args ...interface{}) (security.Caveat, error) {
	var empty security.Caveat
	if len(args) != 1 {
		return empty, fmt.Errorf("expiry caveat: must input exactly one time argument")
	}
	t, ok := args[0].(time.Time)
	if !ok {
		return empty, fmt.Errorf("expiry caveat: received arg of type %T, expected time.Time", args[0])
	}
	return security.NewExpiryCaveat(t)
}

func methodCaveat(args ...interface{}) (security.Caveat, error) {
	if len(args) < 1 {
		return security.Caveat{}, fmt.Errorf("method caveat requires at least one argument")
	}
	methods, err := interfacesToStrings(args)
	if err != nil {
		return security.Caveat{}, fmt.Errorf("method caveat: %v", err)
	}
	return security.NewMethodCaveat(methods[0], methods[1:]...)
}

func peerBlessingsCaveat(args ...interface{}) (security.Caveat, error) {
	if len(args) < 1 {
		return security.Caveat{}, fmt.Errorf("peer-blessings caveat requires at least one argument")
	}
	patterns, err := interfacesToBlessingPatterns(args)
	if err != nil {
		return security.Caveat{}, fmt.Errorf("peer-blessings caveat: %v", err)
	}
	return security.NewCaveat(security.PeerBlessingsCaveat, patterns)
}

func interfacesToStrings(args []interface{}) ([]string, error) {
	var s []string
	for _, arg := range args {
		a, ok := arg.(string)
		if !ok {
			return nil, fmt.Errorf("received arg of type %T, expected string", arg)
		}
		s = append(s, a)
	}
	return s, nil
}

func interfacesToBlessingPatterns(args []interface{}) ([]security.BlessingPattern, error) {
	var bps []security.BlessingPattern
	for _, arg := range args {
		a, ok := arg.(security.BlessingPattern)
		if !ok {
			return nil, fmt.Errorf("received arg of type %T, expected security.BlessingPattern", arg)
		}
		bps = append(bps, a)
	}
	return bps, nil
}

func revocationCaveat(args ...interface{}) (security.Caveat, error) {
	var empty security.Caveat
	if len(args) != 3 {
		return empty, fmt.Errorf("revocation caveat: must input a revocation manager, publickey, and discharge location")
	}
	revocationManager, ok := args[0].(revocation.RevocationManager)
	if !ok {
		return empty, fmt.Errorf("revocation caveat: received args of type %T, expected revocation.RevocationManager", args[0])
	}
	publicKey, ok := args[1].(security.PublicKey)
	if !ok {
		return empty, fmt.Errorf("revocation caveat: received args of type %T, expected security.PublicKey", args[1])
	}
	dischargerLocation, ok := args[2].(string)
	if !ok {
		return empty, fmt.Errorf("revocation caveat: received args of type %T, expected string", args[2])
	}
	return revocationManager.NewCaveat(publicKey, dischargerLocation)
}
