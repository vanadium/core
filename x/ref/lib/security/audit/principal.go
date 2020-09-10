// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package audit

import (
	"fmt"
	"time"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/security"
)

// NewPrincipal returns a security.Principal implementation that logs
// all private key operations of 'wrapped' to 'auditor' (i.e., all calls to
// BlessSelf, Bless, MintDischarge and Sign).
func NewPrincipal(ctx *context.T, auditor Auditor) security.Principal {
	wrapped := v23.GetPrincipal(ctx)
	return &auditingPrincipal{principal: wrapped, auditor: auditor, ctx: ctx}
}

type auditingPrincipal struct {
	principal security.Principal
	auditor   Auditor
	ctx       *context.T
}

type args []interface{}

var noBlessings security.Blessings

func (p *auditingPrincipal) Bless(key security.PublicKey, with security.Blessings, extension string, caveat security.Caveat, additionalCaveats ...security.Caveat) (security.Blessings, error) {
	blessings, err := p.principal.Bless(key, with, extension, caveat, additionalCaveats...)
	if err = p.audit(err, "Bless", addCaveats(args{key, with, extension, caveat}, additionalCaveats...), blessings); err != nil {
		return noBlessings, err
	}
	return blessings, nil
}

func (p *auditingPrincipal) BlessSelf(name string, caveats ...security.Caveat) (security.Blessings, error) {
	blessings, err := p.principal.BlessSelf(name, caveats...)
	if err = p.audit(err, "BlessSelf", addCaveats(args{name}, caveats...), blessings); err != nil {
		return noBlessings, err
	}
	return blessings, nil
}

func (p *auditingPrincipal) Sign(message []byte) (security.Signature, error) {
	// Do not save the signature itself.
	sig, err := p.principal.Sign(message)
	if err = p.audit(err, "Sign", args{message}, nil); err != nil {
		return security.Signature{}, err
	}
	return sig, nil
}

func (p *auditingPrincipal) MintDischarge(forCaveat, caveatOnDischarge security.Caveat, additionalCaveatsOnDischarge ...security.Caveat) (security.Discharge, error) {
	d, err := p.principal.MintDischarge(forCaveat, caveatOnDischarge, additionalCaveatsOnDischarge...)
	// No need to log the discharge
	if err = p.audit(err, "MintDischarge", addCaveats(args{forCaveat, caveatOnDischarge}, additionalCaveatsOnDischarge...), nil); err != nil {
		return security.Discharge{}, err
	}
	return d, nil
}

func (p *auditingPrincipal) PublicKey() security.PublicKey { return p.principal.PublicKey() }
func (p *auditingPrincipal) Roots() security.BlessingRoots { return p.principal.Roots() }
func (p *auditingPrincipal) BlessingStore() security.BlessingStore {
	return p.principal.BlessingStore()
}

func (p *auditingPrincipal) audit(err error, method string, args args, result interface{}) error {
	if err != nil {
		return err
	}
	entry := Entry{Method: method, Timestamp: time.Now()}
	if len(args) > 0 {
		entry.Arguments = []interface{}(args)
	}
	if result != nil {
		entry.Results = []interface{}{result}
	}
	if err := p.auditor.Audit(p.ctx, entry); err != nil {
		return fmt.Errorf("failed to audit call to %v: %v", method, err)
	}
	return nil
}

func addCaveats(args args, caveats ...security.Caveat) args {
	for _, c := range caveats {
		args = append(args, c)
	}
	return args
}
