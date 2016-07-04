// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package signing_test implements a test for the package
// v.io/x/ref/services/syncbase/signing
package signing_test

import "crypto/sha256"
import "testing"
import "time"

import "v.io/x/ref/services/syncbase/signing"
import "v.io/x/ref/services/syncbase/signing/krl"
import "v.io/v23/naming"
import "v.io/v23/security"
import "v.io/v23/vdl"
import "v.io/v23/vom"
import "v.io/v23/verror"
import "v.io/x/ref/test"
import lib_security "v.io/x/ref/lib/security"

import _ "v.io/x/ref/runtime/factories/generic"

// --------------------------------------
// The following implements a fake security.Call.
type fakeCall struct {
	localPrincipal  security.Principal
	localBlessings  security.Blessings
	remoteBlessings security.Blessings
}

func (fc *fakeCall) Timestamp() time.Time                            { return time.Now() }
func (fc *fakeCall) Method() string                                  { return "the_method_name" }
func (fc *fakeCall) MethodTags() []*vdl.Value                        { return nil }
func (fc *fakeCall) Suffix() string                                  { return "the_suffix" }
func (fc *fakeCall) LocalDischarges() map[string]security.Discharge  { return nil }
func (fc *fakeCall) RemoteDischarges() map[string]security.Discharge { return nil }
func (fc *fakeCall) LocalPrincipal() security.Principal              { return fc.localPrincipal }
func (fc *fakeCall) LocalBlessings() security.Blessings              { return fc.localBlessings }
func (fc *fakeCall) RemoteBlessings() security.Blessings             { return fc.remoteBlessings }
func (fc *fakeCall) LocalEndpoint() naming.Endpoint                  { return naming.Endpoint{} }
func (fc *fakeCall) RemoteEndpoint() naming.Endpoint                 { return naming.Endpoint{} }

// --------------------------------------

// A principalDesc holds the local state of a single principal in the tests below.
type principalDesc struct {
	name                string
	principal           security.Principal
	blessings           security.Blessings
	krl                 *krl.KRL
	authorBlessingsData *signing.BlessingsData
	names               []string
	marshalledBlessings []byte
	blessingsHash       []byte
	validatorData       *signing.ValidatorData
	validatorHash       []byte
	cache               *signing.ValidationCache
	data                *signing.DataWithSignature
}

// makePrincipal() returns a pointer to a newly-initialized principalDesc,
// with a unique key, and a single blessing named with its own name.
func makePrincipal(t testing.TB, name string) (desc *principalDesc) {
	var err error
	desc = new(principalDesc)
	desc.name = name
	desc.principal, err = lib_security.NewPrincipal()
	if err != nil {
		t.Fatalf("security.CreatePrincipal %q failed: %v", desc.name, err)
	}
	desc.blessings, err = desc.principal.BlessSelf(desc.name)
	if err != nil {
		t.Fatalf("principal.BlessSelf %q failed: %v", desc.name, err)
	}
	desc.krl = krl.New()
	desc.cache = signing.NewValidationCache(5 * time.Second)
	return desc
}

// makePrincipals() creares one principal per name, and adds
// the blessings of each to the roots of all.
func makePrincipals(t testing.TB, names ...string) (principals []*principalDesc) {
	for i := range names {
		principals = append(principals, makePrincipal(t, names[i]))
	}
	for i := range principals {
		for j := range principals {
			security.AddToRoots(principals[j].principal, principals[i].blessings)
		}
	}
	return principals
}

// BenchmarkHashData() measures the time taken to do a cryptogrqaphic hash of
// 1kBytes.
func BenchmarkHashData(b *testing.B) {
	var block [1024]byte
	hasher := sha256.New()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		hasher.Write(block[:])
	}
}

// BenchmarkSignData() measures the time taken to sign something with
// signing.SignData().
func BenchmarkSignData(b *testing.B) {
	ctx, shutdown := test.V23Init()
	defer shutdown()
	var err error
	author := makePrincipal(b, "author")
	dataToSign := []signing.Item{signing.ItemData{Value: []byte("hello")}}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		author.data, author.authorBlessingsData, err =
			signing.SignData(ctx, author.cache, author.principal, author.blessings, dataToSign)
	}
	if err != nil {
		panic(err)
	}
}

// BenchmarkSign1000Data() measures the time taken to sign 1000 small data
// items with signing.SignData().
func BenchmarkSign1000Data(b *testing.B) {
	ctx, shutdown := test.V23Init()
	defer shutdown()
	var err error
	author := makePrincipal(b, "author")
	var dataToSign []signing.Item
	for i := 0; i != 1000; i++ {
		dataToSign = append(dataToSign, signing.ItemData{Value: []byte("hello")})
	}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		author.data, author.authorBlessingsData, err =
			signing.SignData(ctx, author.cache, author.principal, author.blessings, dataToSign)
	}
	if err != nil {
		panic(err)
	}
}

// BenchmarkSignData() measures the time taken to check a validated signature
// with DataWithSignature.Check().
func BenchmarkCheckData(b *testing.B) {
	ctx, shutdown := test.V23Init()
	defer shutdown()
	var err error

	principals := makePrincipals(b, "author", "validator", "checker")
	author := principals[0]
	validator := principals[1]
	checker := principals[2]

	dataToSign := []signing.Item{signing.ItemData{Value: []byte("hello")}}
	author.data, author.authorBlessingsData, err =
		signing.SignData(ctx, author.cache, author.principal, author.blessings, dataToSign)
	if err != nil {
		panic(err)
	}
	callToValidator := fakeCall{
		localPrincipal:  validator.principal,
		localBlessings:  validator.blessings,
		remoteBlessings: author.blessings,
	}
	validator.names, err = author.data.Check(ctx, author.cache, &callToValidator, validator.krl, 24*30*time.Hour)
	if err != nil {
		panic(err)
	}
	callToChecker := fakeCall{
		localPrincipal:  checker.principal,
		localBlessings:  checker.blessings,
		remoteBlessings: validator.blessings,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		checker.names, err = author.data.Check(ctx, author.cache, &callToChecker, checker.krl, 24*30*time.Hour)
	}
}

// BenchmarkSign1000Data() measures the time taken to check a validated
// signature over 1000 small data items with DataWithSignature.Check().
func BenchmarkCheck1000Data(b *testing.B) {
	ctx, shutdown := test.V23Init()
	defer shutdown()
	var err error

	principals := makePrincipals(b, "author", "validator", "checker")
	author := principals[0]
	validator := principals[1]
	checker := principals[2]

	var dataToSign []signing.Item
	for i := 0; i != 1000; i++ {
		dataToSign = append(dataToSign, signing.ItemData{Value: []byte("hello")})
	}
	author.data, author.authorBlessingsData, err =
		signing.SignData(ctx, author.cache, author.principal, author.blessings, dataToSign)
	if err != nil {
		panic(err)
	}
	callToValidator := fakeCall{
		localPrincipal:  validator.principal,
		localBlessings:  validator.blessings,
		remoteBlessings: author.blessings,
	}
	validator.names, err = author.data.Check(ctx, author.cache, &callToValidator, validator.krl, 24*30*time.Hour)
	if err != nil {
		panic(err)
	}
	callToChecker := fakeCall{
		localPrincipal:  checker.principal,
		localBlessings:  checker.blessings,
		remoteBlessings: validator.blessings,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		checker.names, err = author.data.Check(ctx, author.cache, &callToChecker, checker.krl, 24*30*time.Hour)
	}
}

// BenchmarkMarshallBlessings() measures the time taken to marshal a Blessings.
func BenchmarkMarshallBlessings(b *testing.B) {
	var err error
	author := makePrincipal(b, "author")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		author.marshalledBlessings, err = vom.Encode(author.blessings)
	}
	if err != nil {
		b.Fatalf("vom.Encode failed: %v", err)
	}
}

// BenchmarkUnmarshallBlessings() measures the time taken to unmashal a Blessings.
func BenchmarkUnmarshallBlessings(b *testing.B) {
	var err error
	author := makePrincipal(b, "author")
	author.marshalledBlessings, err = vom.Encode(author.blessings)
	if err != nil {
		b.Fatalf("vom.Encode failed: %v", err)
	}
	var blessings security.Blessings
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err = vom.Decode(author.marshalledBlessings, &blessings)
	}
	if err != nil {
		b.Fatalf("vom.Encode failed: %v", err)
	}
}

// BenchmarkMarshallPublicKey() measures the time taken to marshal a PublicKey.
func BenchmarkMarshallPublicKey(b *testing.B) {
	var err error
	author := makePrincipal(b, "author")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err = author.principal.PublicKey().MarshalBinary()
	}
	if err != nil {
		b.Fatalf("MarshalBinary() failed: %v", err)
	}
}

// BenchmarkUnmarshallPublicKey() measures the time taken to unmarshal a PublicKey.
func BenchmarkUnmarshallPublicKey(b *testing.B) {
	var err error
	author := makePrincipal(b, "author")
	var marshalledKey []byte
	marshalledKey, err = author.principal.PublicKey().MarshalBinary()
	if err != nil {
		b.Fatalf("MarshalBinary() failed: %v", err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err = security.UnmarshalPublicKey(marshalledKey)
	}
	if err != nil {
		b.Fatalf("MarshalBinary() failed: %v", err)
	}
}

// TestSignData() tests that a complete flow of signing, validating, and
// checking works on a DataWithSignature.
func TestSignData(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	var err error

	principals := makePrincipals(t, "author", "validator", "checker")
	author := principals[0]
	validator := principals[1]
	checker := principals[2]

	// Add each princpipal's blessings to each principal's roots.
	pdList := []*principalDesc{author, validator, checker}
	for i := 0; i != len(pdList); i++ {
		for j := 0; j != len(pdList); j++ {
			security.AddToRoots(pdList[j].principal, pdList[i].blessings)
		}
	}

	// --------------------------------------
	// Author
	// Sign some data.
	dataToSign := []signing.Item{
		signing.ItemData{Value: []byte("hello")},
		signing.ItemData{Value: []byte("world")},
		signing.ItemData{Value: []byte("!")},
	}
	author.data, author.authorBlessingsData, err =
		signing.SignData(ctx, author.cache, author.principal, author.blessings, dataToSign)
	if err != nil {
		t.Fatalf("signing.SignData failed: %v", err)
	}
	if author.data.IsValidated {
		t.Fatalf("signing.SignData generated data with IsValidated set")
	}

	// --------------------------------------
	// Validator
	callToValidator := fakeCall{
		localPrincipal:  validator.principal,
		localBlessings:  validator.blessings,
		remoteBlessings: author.blessings,
	}
	// The validator receives author.data from the author.
	validator.data = new(signing.DataWithSignature)
	*validator.data = *author.data
	// Initially the validator doesn't have the author BlessingsData.
	validator.authorBlessingsData = validator.cache.LookupBlessingsData(ctx, validator.data.BlessingsHash)
	if validator.authorBlessingsData != nil {
		t.Errorf("found non-nil BlessingsData for validator.data.BlessingsHash in validator's ValidationCache")
	}
	validator.names, err = validator.data.Check(ctx, validator.cache, &callToValidator, validator.krl, 24*30*time.Hour)
	if verror.ErrorID(err) != signing.ErrNeedAuthorBlessingsForHash.ID {
		t.Fatalf("validator.data.Check got err %v, want %s", err, signing.ErrNeedAuthorBlessingsForHash.ID)
	}

	// The validator receives the author's marshalled blessings from the author.
	validator.marshalledBlessings = author.authorBlessingsData.MarshalledBlessings
	validator.blessingsHash, validator.authorBlessingsData, err = validator.cache.AddWireBlessings(ctx, validator.marshalledBlessings)
	if err != nil {
		t.Fatalf("validator can't add author's marshalled belssings to its ValidationCache: %v", err)
	}

	validator.names, err = validator.data.Check(ctx, validator.cache, &callToValidator, validator.krl, 24*30*time.Hour)
	if err != nil {
		t.Fatalf("validator error calling Check() on data: %v", err)
	}
	if !validator.data.IsValidated {
		t.Fatalf("signing.Check didn't set IsValidated")
	}
	// Validator's cache should now have the author's BlessingData, and the validator's ValidatorData.
	validator.authorBlessingsData = validator.cache.LookupBlessingsData(ctx, validator.data.BlessingsHash)
	if validator.authorBlessingsData == nil {
		t.Errorf("didn't finf BlessingsData for validator.data.BlessingsHash in validator's ValidationCache")
	}
	validator.validatorData = validator.cache.LookupValidatorData(ctx, validator.data.ValidatorDataHash)

	// --------------------------------------
	// Checker
	callToChecker := fakeCall{
		localPrincipal:  checker.principal,
		localBlessings:  checker.blessings,
		remoteBlessings: validator.blessings,
	}
	// The checker recieves validator.data from the validator, except that
	// data item 1 is replaced by its hash, because (for example) the
	// check is not allowed to see it.
	checker.data = new(signing.DataWithSignature)
	*checker.data = *validator.data
	checker.data.Data[1] = signing.ItemHash{Value: signing.SumByteVectorWithLength(checker.data.Data[1].(signing.ItemData).Value)}

	// Initially the checker doesn't have the author BlessingsData, or the validator ValidatorData.
	checker.authorBlessingsData = checker.cache.LookupBlessingsData(ctx, checker.data.BlessingsHash)
	if checker.authorBlessingsData != nil {
		t.Errorf("found non-nil blessings data for checker.data.BlessingsHash hash in checker's ValidationCache")
	}
	checker.names, err = checker.data.Check(ctx, checker.cache, &callToChecker, checker.krl, 24*30*time.Hour)
	if verror.ErrorID(err) != signing.ErrNeedAuthorBlessingsAndValidatorDataForHash.ID {
		t.Fatalf("checker.data.Check got err %v, want %s", err, signing.ErrNeedAuthorBlessingsAndValidatorDataForHash.ID)
	}

	// The checker receives the author's marshalled blessings from the validator.
	checker.marshalledBlessings = validator.marshalledBlessings
	checker.blessingsHash, checker.authorBlessingsData, err = checker.cache.AddWireBlessings(ctx, checker.marshalledBlessings)
	if err != nil {
		t.Fatalf("checker can't add author's marshalled belssings to its ValidationCache: %v", err)
	}
	checker.names, err = checker.data.Check(ctx, checker.cache, &callToChecker, checker.krl, 24*30*time.Hour)
	if verror.ErrorID(err) != signing.ErrNeedValidatorDataForHash.ID {
		t.Fatalf("checker.data.Check got err %v, want %s", err, signing.ErrNeedValidatorDataForHash.ID)
	}

	// The checker receives the validator's data from the validator, passing through the wire format.
	wvd := signing.ToWireValidatorData(validator.validatorData)
	var vd signing.ValidatorData
	vd, err = signing.FromWireValidatorData(&wvd)
	if err != nil {
		t.Fatalf("signing.FromWireValidatorData got error:  %v", err)
	}
	checker.validatorData = &vd

	// The checker adds the ValidatorData to its cache.
	checker.validatorHash = checker.cache.AddValidatorData(ctx, checker.validatorData)

	// And now the Check() operation should work.
	checker.names, err = checker.data.Check(ctx, checker.cache, &callToChecker, checker.krl, 24*30*time.Hour)
	if err != nil {
		t.Fatalf("checker.data.Check got unexpected err %v", err)
	}
}
