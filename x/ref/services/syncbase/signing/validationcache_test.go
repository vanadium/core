// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file tests the validationcache.go module.

package signing_test

import "bytes"
import "testing"
import "time"

import "v.io/x/ref/services/syncbase/signing"
import "v.io/v23/security"
import "v.io/x/ref/test"
import _ "v.io/x/ref/runtime/factories/generic"
import lib_security "v.io/x/ref/lib/security"

// A principalVDesc holds the local state of a single principal in the tests below.
type principalVDesc struct {
	name          string
	principal     security.Principal
	blessings     security.Blessings
	blessingsHash []byte
	blessingsData *signing.BlessingsData
	validatorHash []byte
	validatorData *signing.ValidatorData
	cache         *signing.ValidationCache
}

// makePrincipalVDesc() returns a pointer to a newly-initialized principalVDesc,
// with a unique key, and a single blessing named with its own name.
func makePrincipalVDesc(t *testing.T, name string) (desc *principalVDesc) {
	var err error
	desc = new(principalVDesc)
	desc.name = name
	desc.principal, err = lib_security.NewPrincipal()
	if err != nil {
		t.Fatalf("lib_security.NewPrincipal %q failed: %v", desc.name, err)
	}
	desc.blessings, err = desc.principal.BlessSelf(desc.name)
	if err != nil {
		t.Fatalf("principal.BlessSelf %q failed: %v", desc.name, err)
	}
	desc.cache = signing.NewValidationCache(5 * time.Second)
	return desc
}

func TestValidationCache(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	var err error

	// Make a principalVDesc for each of the author, validator, and checker.
	// (The author creates a signed change; the validator is a device
	// author syncs with; the checker is a device a validator syncs with.)
	author := makePrincipalVDesc(t, "author")
	validator := makePrincipalVDesc(t, "validator")
	checker := makePrincipalVDesc(t, "checker")

	// Add each princpipal's blessings to each principal's roots.
	pdList := []*principalVDesc{author, validator, checker}
	for i := 0; i != len(pdList); i++ {
		for j := 0; j != len(pdList); j++ {
			security.AddToRoots(pdList[j].principal, pdList[i].blessings)
		}
	}

	// --------------------------------------
	// Author
	arbitraryBlessingsData := author.cache.LookupBlessingsData(ctx, []byte{0x00})
	if arbitraryBlessingsData != nil {
		t.Errorf("found non-nil blessings data for nonsense hash in author's ValidationCache")
	}
	author.blessingsHash, author.blessingsData, err = author.cache.AddBlessings(ctx, author.blessings)
	if err != nil {
		t.Fatalf("error from author.cache.AddBlessings(): %v", err)
	}
	// Check that the author's data is as we expect.
	if author.cache.LookupBlessingsData(ctx, author.blessingsHash) != author.blessingsData {
		t.Fatalf("found wrong blessings data for hash in author's ValidationCache: %v vs %v",
			author.cache.LookupBlessingsData(ctx, author.blessingsHash), author.blessingsData)
	}

	// --------------------------------------
	// Validator
	// The validator receives author.blessingsHash from the author.
	// Initially the validator doesn't have the author BlessingsData.
	authorBlessingsData := validator.cache.LookupBlessingsData(ctx, author.blessingsHash)
	if authorBlessingsData != nil {
		t.Errorf("found non-nil blessings data for author.blessingsHash hash in validator's ValidationCache")
	}
	// The validator receives the author's marshalled blessings from the author.
	validator.blessingsHash, validator.blessingsData, err =
		validator.cache.AddWireBlessings(ctx, author.blessingsData.MarshalledBlessings)
	if err != nil {
		t.Fatalf("validator can't add author's marshalled blessings to its ValidationCache: %v", err)
	}
	if !bytes.Equal(author.blessingsHash, validator.blessingsHash) {
		t.Errorf("validator's copy of the blessingsHash different from author's")
	}
	// Check that we could have got the blessingsData with a lookup if this were the second time.
	if validator.cache.LookupBlessingsData(ctx, validator.blessingsHash) != validator.blessingsData {
		t.Fatalf("found wrong blessings data for hash in validator's ValidationCache")
	}
	var marshalledPublicKey []byte
	marshalledPublicKey, err = validator.principal.PublicKey().MarshalBinary()
	if err != nil {
		t.Fatalf("validator.principal.PublicKey().MarshalBinary() got error: %v", err)
	}

	var validatedNames []string
	validatedNames, _ = security.SigningBlessingNames(ctx, validator.principal,
		validator.blessingsData.UnmarshalledBlessings)
	validator.validatorData = &signing.ValidatorData{
		Names:               validatedNames,
		PublicKey:           validator.principal.PublicKey(),
		MarshalledPublicKey: marshalledPublicKey}
	validator.validatorHash = validator.cache.AddValidatorData(ctx, validator.validatorData)
	if validator.cache.LookupValidatorData(ctx, validator.validatorHash) != validator.validatorData {
		t.Fatalf("LookupValidatorData returned wrong ValidatorData pointer in validator")
	}

	// --------------------------------------
	// Checker
	// The checker receives validator.blessingsHash from the validator.
	// Initially the checker doesn't have the author BlessingsData.
	authorBlessingsData = checker.cache.LookupBlessingsData(ctx, validator.blessingsHash)
	if authorBlessingsData != nil {
		t.Errorf("found non-nil blessings data for author.blessingsHash hash in checker's ValidationCache")
	}
	// The checker receives the author's marshalled blessings from the validator.
	checker.blessingsHash, checker.blessingsData, err =
		checker.cache.AddWireBlessings(ctx, validator.blessingsData.MarshalledBlessings)
	if err != nil {
		t.Fatalf("checker can't add author's marshalled blessings (from validator) to ValidationCache: %v", err)
	}
	if !bytes.Equal(author.blessingsHash, checker.blessingsHash) {
		t.Errorf("checker's copy of the blessingsHash different from author's")
	}
	// Check that we could have got the blessingsData with a lookup if this where the second time.
	if checker.cache.LookupBlessingsData(ctx, checker.blessingsHash) != checker.blessingsData {
		t.Fatalf("found wrong blessings data for hash in checker's ValidationCache")
	}
	// The checker recieves validator.validatorHash from the validator.
	// Initially the checker doesn't have the ValidatorData.
	validatorData := checker.cache.LookupValidatorData(ctx, validator.validatorHash)
	if validatorData != nil {
		t.Errorf("found non-nil validator data for validator.validatorHash hash in checker's ValidationCache")
	}
	// The checker receives the validator's data from the validator (or another checker).
	checker.validatorHash = checker.cache.AddValidatorData(ctx, validator.validatorData)
	if !bytes.Equal(validator.validatorHash, checker.validatorHash) {
		t.Fatalf("checker's copy of the validatorHash different from validator's")
	}
	// Get the validatorData
	checker.validatorData = checker.cache.LookupValidatorData(ctx, checker.validatorHash)
	if checker.validatorData == nil {
		t.Fatalf("found nil valdidatorData for checker.validatorHash hash in checker's ValidationCache")
	}
}

func TestWireValidatorData(t *testing.T) {
	var err error

	pDesc := makePrincipalVDesc(t, "some_principal")

	var vd signing.ValidatorData
	vd.Names = []string{"wombat", "foo"}
	vd.PublicKey = pDesc.principal.PublicKey()
	vd.MarshalledPublicKey, err = vd.PublicKey.MarshalBinary()
	if err != nil {
		t.Fatalf("failed to marshel public key: %v\n", err)
	}

	var wvd signing.WireValidatorData
	var vd2 signing.ValidatorData

	wvd = signing.ToWireValidatorData(&vd)
	vd2, err = signing.FromWireValidatorData(&wvd)
	if err != nil {
		t.Fatalf("FromWireValidatorData failed: %v\n", err)
	}
	if len(vd.Names) != len(vd2.Names) {
		t.Fatalf("ToWireValidatorData/FromWireValidatorData failed to transfer Names list correctly:\nold\n%v\n\nnew\n%v\n\nwire\n%v\n",
			vd, vd2, wvd)
	}
}
