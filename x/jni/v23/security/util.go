// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build java android

package security

import (
	"v.io/v23/security"
	"v.io/v23/uniqueid"
	"v.io/v23/vom"

	jutil "v.io/x/jni/util"
)

// #include "jni.h"
import "C"

// JavaBlessings converts the provided Go Blessings into Java Blessings.
func JavaBlessings(env jutil.Env, blessings security.Blessings) (jutil.Object, error) {
	ref := jutil.GoNewRef(&blessings) // Un-refed when the Java Blessings object is finalized.
	jBlessings, err := jutil.NewObject(env, jBlessingsClass, []jutil.Sign{jutil.LongSign}, int64(ref))
	if err != nil {
		jutil.GoDecRef(ref)
		return jutil.NullObject, err
	}
	return jBlessings, nil
}

// GoBlessings converts the provided Java Blessings into Go Blessings.
func GoBlessings(env jutil.Env, jBlessings jutil.Object) (security.Blessings, error) {
	if jBlessings.IsNull() {
		return security.Blessings{}, nil
	}
	ref, err := jutil.CallLongMethod(env, jBlessings, "nativeRef", nil)
	if err != nil {
		return security.Blessings{}, err
	}
	return (*(*security.Blessings)(jutil.GoRefValue(jutil.Ref(ref)))), nil
}

// GoBlessingsArray converts the provided Java Blessings array into a Go
// Blessings slice.
func GoBlessingsArray(env jutil.Env, jBlessingsArr jutil.Object) ([]security.Blessings, error) {
	barr, err := jutil.GoObjectArray(env, jBlessingsArr)
	if err != nil {
		return nil, err
	}
	ret := make([]security.Blessings, len(barr))
	for i, jBlessings := range barr {
		var err error
		if ret[i], err = GoBlessings(env, jBlessings); err != nil {
			return nil, err
		}
	}
	return ret, nil
}

// JavaWireBlessings converts the provided Go WireBlessings into Java WireBlessings.
func JavaWireBlessings(env jutil.Env, wire security.WireBlessings) (jutil.Object, error) {
	return jutil.JVomCopy(env, wire, jWireBlessingsClass)
}

// JavaCaveat converts the provided Go Caveat into a Java Caveat.
func JavaCaveat(env jutil.Env, caveat security.Caveat) (jutil.Object, error) {
	// NOTE(spetrovic): We could call JVomCopy here, but it's painfully slow and this code is
	// on the critical path.

	// Copy the Id field.
	jId, err := jutil.NewObject(env, jIdClass, []jutil.Sign{jutil.ByteArraySign}, []byte(caveat.Id[:]))
	if err != nil {
		return jutil.NullObject, err
	}
	return jutil.NewObject(env, jCaveatClass, []jutil.Sign{idSign, jutil.ByteArraySign}, jId, caveat.ParamVom)
}

// GoCaveat converts the provided Java Caveat into a Go Caveat.
func GoCaveat(env jutil.Env, jCav jutil.Object) (security.Caveat, error) {
	// NOTE(spetrovic): We could call GoVomCopy here, but it's painfully slow and this code is
	// on the critical path.

	// Copy the Id field.
	jId, err := jutil.CallObjectMethod(env, jCav, "getId", nil, idSign)
	if err != nil {
		return security.Caveat{}, err
	}
	idBytes, err := jutil.CallByteArrayMethod(env, jId, "toPrimitiveArray", nil)
	if err != nil {
		return security.Caveat{}, err
	}
	var id uniqueid.Id
	copy(id[:], idBytes[:len(id)])

	// Copy the ParamVom field.
	paramVom, err := jutil.CallByteArrayMethod(env, jCav, "getParamVom", nil)
	if err != nil {
		return security.Caveat{}, err
	}
	return security.Caveat{
		Id:       id,
		ParamVom: paramVom,
	}, nil
}

// JavaCaveatArray converts the provided Go Caveat slice into a Java Caveat array.
func JavaCaveatArray(env jutil.Env, caveats []security.Caveat) (jutil.Object, error) {
	if caveats == nil {
		return jutil.NullObject, nil
	}
	cavArr := make([]jutil.Object, len(caveats))
	for i, caveat := range caveats {
		var err error
		if cavArr[i], err = JavaCaveat(env, caveat); err != nil {
			return jutil.NullObject, err
		}
	}
	return jutil.JObjectArray(env, cavArr, jCaveatClass)
}

// GoCaveats converts the provided Java Caveat array into a Go Caveat slice.
func GoCaveats(env jutil.Env, jCaveats jutil.Object) ([]security.Caveat, error) {
	cavArr, err := jutil.GoObjectArray(env, jCaveats)
	if err != nil {
		return nil, err
	}
	ret := make([]security.Caveat, len(cavArr))
	for i, jCav := range cavArr {
		var err error
		if ret[i], err = GoCaveat(env, jCav); err != nil {
			return nil, err
		}
	}
	return ret, nil
}

// JavaBlessingPattern converts the provided Go BlessingPattern into Java
// BlessingPattern.
func JavaBlessingPattern(env jutil.Env, pattern security.BlessingPattern) (jutil.Object, error) {
	ref := jutil.GoNewRef(&pattern) // Un-refed when the Java BlessingRootsImpl is finalized.
	jPattern, err := jutil.NewObject(env, jBlessingPatternClass, []jutil.Sign{jutil.LongSign, jutil.StringSign}, int64(ref), string(pattern))
	if err != nil {
		jutil.GoDecRef(ref)
		return jutil.NullObject, err
	}
	return jPattern, nil
}

// GoBlessingPattern converts the provided Java BlessingPattern into Go BlessingPattern.
func GoBlessingPattern(env jutil.Env, jPattern jutil.Object) (pattern security.BlessingPattern, err error) {
	if jPattern.IsNull() {
		return "", nil
	}
	ref, err := jutil.CallLongMethod(env, jPattern, "nativeRef", nil)
	if err != nil {
		return "", err
	}
	return (*(*security.BlessingPattern)(jutil.GoRefValue(jutil.Ref(ref)))), nil
}

// JavaPublicKey converts the provided Go PublicKey into Java PublicKey.
func JavaPublicKey(env jutil.Env, key security.PublicKey) (jutil.Object, error) {
	if key == nil {
		return jutil.NullObject, nil
	}
	der, err := key.MarshalBinary()
	if err != nil {
		return jutil.NullObject, err
	}
	return JavaPublicKeyFromDER(env, der)
}

// JavaPublicKeyFromDER converts a DER-encoded public key into a Java PublicKey object.
func JavaPublicKeyFromDER(env jutil.Env, der []byte) (jutil.Object, error) {
	jPublicKey, err := jutil.CallStaticObjectMethod(env, jUtilClass, "decodePublicKey", []jutil.Sign{jutil.ArraySign(jutil.ByteSign)}, publicKeySign, der)
	if err != nil {
		return jutil.NullObject, err
	}
	return jPublicKey, nil
}

// JavaPublicKeyToDER returns the DER-encoded representations of a Java PublicKey object.
func JavaPublicKeyToDER(env jutil.Env, jKey jutil.Object) ([]byte, error) {
	return jutil.CallStaticByteArrayMethod(env, jUtilClass, "encodePublicKey", []jutil.Sign{publicKeySign}, jKey)
}

// GoPublicKey converts the provided Java PublicKey into Go PublicKey.
func GoPublicKey(env jutil.Env, jKey jutil.Object) (security.PublicKey, error) {
	der, err := JavaPublicKeyToDER(env, jKey)
	if err != nil {
		return nil, err
	}
	return security.UnmarshalPublicKey(der)
}

// JavaSignature converts the provided Go Signature into a Java VSignature.
func JavaSignature(env jutil.Env, sig security.Signature) (jutil.Object, error) {
	encoded, err := vom.Encode(sig)
	if err != nil {
		return jutil.NullObject, err
	}
	jSignature, err := jutil.CallStaticObjectMethod(env, jUtilClass, "decodeSignature", []jutil.Sign{jutil.ByteArraySign}, signatureSign, encoded)
	if err != nil {
		return jutil.NullObject, err
	}
	return jSignature, nil
}

// GoSignature converts the provided Java VSignature into a Go Signature.
func GoSignature(env jutil.Env, jSignature jutil.Object) (security.Signature, error) {
	encoded, err := jutil.CallStaticByteArrayMethod(env, jUtilClass, "encodeSignature", []jutil.Sign{signatureSign}, jSignature)
	if err != nil {
		return security.Signature{}, err
	}
	var sig security.Signature
	if err := vom.Decode(encoded, &sig); err != nil {
		return security.Signature{}, err
	}
	return sig, nil
}

// GoDischarge converts the provided Java Discharge into a Go Discharge.
func GoDischarge(env jutil.Env, jDischarge jutil.Object) (security.Discharge, error) {
	var discharge security.Discharge
	if err := jutil.GoVomCopy(env, jDischarge, jDischargeClass, &discharge); err != nil {
		return security.Discharge{}, err
	}
	return discharge, nil
}

// JavaDischarge converts the provided Go Discharge into a Java discharge.
func JavaDischarge(env jutil.Env, discharge security.Discharge) (jutil.Object, error) {
	return jutil.JVomCopy(env, discharge, jDischargeClass)
}

func javaDischargeMap(env jutil.Env, discharges map[string]security.Discharge) (jutil.Object, error) {
	objectMap := make(map[jutil.Object]jutil.Object)
	for key, discharge := range discharges {
		jKey := jutil.JString(env, key)
		jDischarge, err := JavaDischarge(env, discharge)
		if err != nil {
			return jutil.NullObject, err
		}
		objectMap[jKey] = jDischarge
	}
	return jutil.JObjectMap(env, objectMap)
}
