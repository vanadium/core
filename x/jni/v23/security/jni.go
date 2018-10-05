// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build java android

package security

import (
	"fmt"
	"unsafe"

	"v.io/v23/security"
	jutil "v.io/x/jni/util"
	jcontext "v.io/x/jni/v23/context"
	vsecurity "v.io/x/ref/lib/security"
)

// #include "jni.h"
import "C"

var (
	contextSign          = jutil.ClassSign("io.v.v23.context.VContext")
	dischargeSign        = jutil.ClassSign("io.v.v23.security.Discharge")
	dischargeImpetusSign = jutil.ClassSign("io.v.v23.security.DischargeImpetus")
	principalSign        = jutil.ClassSign("io.v.v23.security.VPrincipal")
	blessingsSign        = jutil.ClassSign("io.v.v23.security.Blessings")
	wireBlessingsSign    = jutil.ClassSign("io.v.v23.security.WireBlessings")
	wireDischargeSign    = jutil.ClassSign("io.v.v23.security.WireDischarge")
	blessingStoreSign    = jutil.ClassSign("io.v.v23.security.BlessingStore")
	blessingRootsSign    = jutil.ClassSign("io.v.v23.security.BlessingRoots")
	blessingPatternSign  = jutil.ClassSign("io.v.v23.security.BlessingPattern")
	signerSign           = jutil.ClassSign("io.v.v23.security.VSigner")
	caveatSign           = jutil.ClassSign("io.v.v23.security.Caveat")
	callSign             = jutil.ClassSign("io.v.v23.security.Call")
	signatureSign        = jutil.ClassSign("io.v.v23.security.VSignature")
	idSign               = jutil.ClassSign("io.v.v23.uniqueid.Id")
	publicKeySign        = jutil.ClassSign("java.security.interfaces.ECPublicKey")

	// Global reference for io.v.v23.security.Blessings class.
	jBlessingsClass jutil.Class
	// Global reference for io.v.v23.security.Caveat class.
	jCaveatClass jutil.Class
	// Global reference for io.v.v23.security.WireBlessings class.
	jWireBlessingsClass jutil.Class
	// Global reference for io.v.v23.security.VPrincipalImpl class.
	jVPrincipalImplClass jutil.Class
	// Global reference for io.v.v23.security.BlessingStoreImpl class.
	jBlessingStoreImplClass jutil.Class
	// Global reference for io.v.v23.security.BlessingRootsImpl class.
	jBlessingRootsImplClass jutil.Class
	// Global reference for io.v.v23.security.CallImpl class.
	jCallImplClass jutil.Class
	// Global reference for io.v.v23.security.BlessingPattern class.
	jBlessingPatternClass jutil.Class
	// Global reference for io.v.v23.security.CaveatRegistry class.
	jCaveatRegistryClass jutil.Class
	// Global reference for io.v.v23.security.Util class.
	jUtilClass jutil.Class
	// Global reference for io.v.v23.security.VSecurity class.
	jVSecurityClass jutil.Class
	// Global reference for io.v.v23.security.Discharge class.
	jDischargeClass jutil.Class
	// Global reference for io.v.v23.security.DischargeImpetus class.
	jDischargeImpetusClass jutil.Class
	// Global reference for io.v.v23.security.access.PermissionsAuthorizer class.
	jPermissionsAuthorizerClass jutil.Class
	// Global reference for io.v.v23.uniqueid.Id class.
	jIdClass jutil.Class
	// Global reference for java.lang.Object class.
	jObjectClass jutil.Class
)

// Init initializes the JNI code with the given Java evironment. This method
// must be called from the main Java thread.
func Init(env jutil.Env) error {
	security.OverrideCaveatValidation(caveatValidator)

	// Cache global references to all Java classes used by the package.  This is
	// necessary because JNI gets access to the class loader only in the system
	// thread, so we aren't able to invoke FindClass in other threads.
	var err error
	jBlessingsClass, err = jutil.JFindClass(env, "io/v/v23/security/Blessings")
	if err != nil {
		return err
	}
	jCaveatClass, err = jutil.JFindClass(env, "io/v/v23/security/Caveat")
	if err != nil {
		return err
	}
	jWireBlessingsClass, err = jutil.JFindClass(env, "io/v/v23/security/WireBlessings")
	if err != nil {
		return err
	}
	jVPrincipalImplClass, err = jutil.JFindClass(env, "io/v/v23/security/VPrincipalImpl")
	if err != nil {
		return err
	}
	jBlessingStoreImplClass, err = jutil.JFindClass(env, "io/v/v23/security/BlessingStoreImpl")
	if err != nil {
		return err
	}
	jBlessingRootsImplClass, err = jutil.JFindClass(env, "io/v/v23/security/BlessingRootsImpl")
	if err != nil {
		return err
	}
	jCallImplClass, err = jutil.JFindClass(env, "io/v/v23/security/CallImpl")
	if err != nil {
		return err
	}
	jBlessingPatternClass, err = jutil.JFindClass(env, "io/v/v23/security/BlessingPattern")
	if err != nil {
		return err
	}
	jCaveatRegistryClass, err = jutil.JFindClass(env, "io/v/v23/security/CaveatRegistry")
	if err != nil {
		return err
	}
	jUtilClass, err = jutil.JFindClass(env, "io/v/v23/security/Util")
	if err != nil {
		return err
	}
	jVSecurityClass, err = jutil.JFindClass(env, "io/v/v23/security/VSecurity")
	if err != nil {
		return err
	}
	jDischargeClass, err = jutil.JFindClass(env, "io/v/v23/security/Discharge")
	if err != nil {
		return err
	}
	jDischargeImpetusClass, err = jutil.JFindClass(env, "io/v/v23/security/DischargeImpetus")
	if err != nil {
		return err
	}
	jPermissionsAuthorizerClass, err = jutil.JFindClass(env, "io/v/v23/security/access/PermissionsAuthorizer")
	if err != nil {
		return err
	}
	jIdClass, err = jutil.JFindClass(env, "io/v/v23/uniqueid/Id")
	if err != nil {
		return err
	}
	jObjectClass, err = jutil.JFindClass(env, "java/lang/Object")
	if err != nil {
		return err
	}
	return nil
}

//export Java_io_v_v23_security_CallImpl_nativeTimestamp
func Java_io_v_v23_security_CallImpl_nativeTimestamp(jenv *C.JNIEnv, jCall C.jobject, goRef C.jlong) C.jobject {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	t := (*(*security.Call)(jutil.GoRefValue(jutil.Ref(goRef)))).Timestamp()
	jTime, err := jutil.JTime(env, t)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	return C.jobject(unsafe.Pointer(jTime))
}

//export Java_io_v_v23_security_CallImpl_nativeMethod
func Java_io_v_v23_security_CallImpl_nativeMethod(jenv *C.JNIEnv, jCall C.jobject, goRef C.jlong) C.jstring {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	method := (*(*security.Call)(jutil.GoRefValue(jutil.Ref(goRef)))).Method()
	jMethod := jutil.JString(env, jutil.CamelCase(method))
	return C.jstring(unsafe.Pointer(jMethod))
}

//export Java_io_v_v23_security_CallImpl_nativeMethodTags
func Java_io_v_v23_security_CallImpl_nativeMethodTags(jenv *C.JNIEnv, jCall C.jobject, goRef C.jlong) C.jobjectArray {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	tags := (*(*security.Call)(jutil.GoRefValue(jutil.Ref(goRef)))).MethodTags()
	jTags, err := jutil.JVDLValueArray(env, tags)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	return C.jobjectArray(unsafe.Pointer(jTags))
}

//export Java_io_v_v23_security_CallImpl_nativeSuffix
func Java_io_v_v23_security_CallImpl_nativeSuffix(jenv *C.JNIEnv, jCall C.jobject, goRef C.jlong) C.jstring {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	jSuffix := jutil.JString(env, (*(*security.Call)(jutil.GoRefValue(jutil.Ref(goRef)))).Suffix())
	return C.jstring(unsafe.Pointer(jSuffix))
}

//export Java_io_v_v23_security_CallImpl_nativeRemoteDischarges
func Java_io_v_v23_security_CallImpl_nativeRemoteDischarges(jenv *C.JNIEnv, jCall C.jobject, goRef C.jlong) C.jobject {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	remoteDischarges := (*(*security.Call)(jutil.GoRefValue(jutil.Ref(goRef)))).RemoteDischarges()
	jObjectMap, err := javaDischargeMap(env, remoteDischarges)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	return C.jobject(unsafe.Pointer(jObjectMap))
}

//export Java_io_v_v23_security_CallImpl_nativeLocalDischarges
func Java_io_v_v23_security_CallImpl_nativeLocalDischarges(jenv *C.JNIEnv, jCall C.jobject, goRef C.jlong) C.jobject {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	localDischarges := (*(*security.Call)(jutil.GoRefValue(jutil.Ref(goRef)))).LocalDischarges()
	jObjectMap, err := javaDischargeMap(env, localDischarges)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	return C.jobject(unsafe.Pointer(jObjectMap))
}

//export Java_io_v_v23_security_CallImpl_nativeLocalEndpoint
func Java_io_v_v23_security_CallImpl_nativeLocalEndpoint(jenv *C.JNIEnv, jCall C.jobject, goRef C.jlong) C.jstring {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	jEndpoint := jutil.JString(env, (*(*security.Call)(jutil.GoRefValue(jutil.Ref(goRef)))).LocalEndpoint().String())
	return C.jstring(unsafe.Pointer(jEndpoint))
}

//export Java_io_v_v23_security_CallImpl_nativeRemoteEndpoint
func Java_io_v_v23_security_CallImpl_nativeRemoteEndpoint(jenv *C.JNIEnv, jCall C.jobject, goRef C.jlong) C.jstring {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	jEndpoint := jutil.JString(env, (*(*security.Call)(jutil.GoRefValue(jutil.Ref(goRef)))).RemoteEndpoint().String())
	return C.jstring(unsafe.Pointer(jEndpoint))

}

//export Java_io_v_v23_security_CallImpl_nativeLocalPrincipal
func Java_io_v_v23_security_CallImpl_nativeLocalPrincipal(jenv *C.JNIEnv, jCall C.jobject, goRef C.jlong) C.jobject {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	principal := (*(*security.Call)(jutil.GoRefValue(jutil.Ref(goRef)))).LocalPrincipal()
	jPrincipal, err := JavaPrincipal(env, principal)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	return C.jobject(unsafe.Pointer(jPrincipal))
}

//export Java_io_v_v23_security_CallImpl_nativeLocalBlessings
func Java_io_v_v23_security_CallImpl_nativeLocalBlessings(jenv *C.JNIEnv, jCall C.jobject, goRef C.jlong) C.jobject {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	blessings := (*(*security.Call)(jutil.GoRefValue(jutil.Ref(goRef)))).LocalBlessings()
	jBlessings, err := JavaBlessings(env, blessings)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	return C.jobject(unsafe.Pointer(jBlessings))
}

//export Java_io_v_v23_security_CallImpl_nativeRemoteBlessings
func Java_io_v_v23_security_CallImpl_nativeRemoteBlessings(jenv *C.JNIEnv, jCall C.jobject, goRef C.jlong) C.jobject {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	blessings := (*(*security.Call)(jutil.GoRefValue(jutil.Ref(goRef)))).RemoteBlessings()
	jBlessings, err := JavaBlessings(env, blessings)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	return C.jobject(unsafe.Pointer(jBlessings))
}

//export Java_io_v_v23_security_CallImpl_nativeFinalize
func Java_io_v_v23_security_CallImpl_nativeFinalize(jenv *C.JNIEnv, jCall C.jobject, goRef C.jlong) {
	jutil.GoDecRef(jutil.Ref(goRef))
}

//export Java_io_v_v23_security_VPrincipalImpl_nativeCreate
func Java_io_v_v23_security_VPrincipalImpl_nativeCreate(jenv *C.JNIEnv, jclass C.jclass) C.jobject {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	principal, err := vsecurity.NewPrincipal()
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	jPrincipal, err := JavaPrincipal(env, principal)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	return C.jobject(unsafe.Pointer(jPrincipal))
}

//export Java_io_v_v23_security_VPrincipalImpl_nativeCreateForSigner
func Java_io_v_v23_security_VPrincipalImpl_nativeCreateForSigner(jenv *C.JNIEnv, jclass C.jclass, jSigner C.jobject) C.jobject {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	signerObj := jutil.Object(uintptr(unsafe.Pointer(jSigner)))
	signer, err := GoSigner(env, signerObj)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	principal, err := vsecurity.NewPrincipalFromSigner(signer, nil)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	ref := jutil.GoNewRef(&principal) // Un-refed when the Java VPrincipalImpl is finalized.
	jPrincipal, err := jutil.NewObject(env, jVPrincipalImplClass, []jutil.Sign{jutil.LongSign, signerSign, blessingStoreSign, blessingRootsSign}, int64(ref), signerObj, jutil.NullObject, jutil.NullObject)
	if err != nil {
		jutil.GoDecRef(ref)
		jutil.JThrowV(env, err)
		return 0
	}
	return C.jobject(unsafe.Pointer(jPrincipal))
}

//export Java_io_v_v23_security_VPrincipalImpl_nativeCreateForAll
func Java_io_v_v23_security_VPrincipalImpl_nativeCreateForAll(jenv *C.JNIEnv, jclass C.jclass, jSigner C.jobject, jStore C.jobject, jRoots C.jobject) C.jobject {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	signerObj := jutil.Object(uintptr(unsafe.Pointer(jSigner)))
	storeObj := jutil.Object(uintptr(unsafe.Pointer(jStore)))
	rootsObj := jutil.Object(uintptr(unsafe.Pointer(jRoots)))
	signer, err := GoSigner(env, signerObj)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	store, err := GoBlessingStore(env, storeObj)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	roots, err := GoBlessingRoots(env, rootsObj)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	// TODO(ataly): Implement BlessingsBasedEncrypter and BlessingsBasedDecrypter types
	// in Java.
	principal, err := security.CreatePrincipal(signer, store, roots)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	ref := jutil.GoNewRef(&principal) // Un-refed when the Java VPrincipalImpl is finalized.
	jPrincipal, err := jutil.NewObject(env, jVPrincipalImplClass, []jutil.Sign{jutil.LongSign, signerSign, blessingStoreSign, blessingRootsSign}, int64(ref), signerObj, storeObj, rootsObj)
	if err != nil {
		jutil.GoDecRef(ref)
		jutil.JThrowV(env, err)
		return 0
	}
	return C.jobject(unsafe.Pointer(jPrincipal))
}

//export Java_io_v_v23_security_VPrincipalImpl_nativeCreatePersistent
func Java_io_v_v23_security_VPrincipalImpl_nativeCreatePersistent(jenv *C.JNIEnv, jclass C.jclass, jPassphrase C.jstring, jDir C.jstring) C.jobject {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	passphrase := jutil.GoString(env, jutil.Object(uintptr(unsafe.Pointer(jPassphrase))))
	dir := jutil.GoString(env, jutil.Object(uintptr(unsafe.Pointer(jDir))))
	principal, err := vsecurity.LoadPersistentPrincipal(dir, []byte(passphrase))
	if err != nil {
		if principal, err = vsecurity.CreatePersistentPrincipal(dir, []byte(passphrase)); err != nil {
			jutil.JThrowV(env, err)
			return 0
		}
	}
	jPrincipal, err := JavaPrincipal(env, principal)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	return C.jobject(unsafe.Pointer(jPrincipal))
}

//export Java_io_v_v23_security_VPrincipalImpl_nativeCreatePersistentForSigner
func Java_io_v_v23_security_VPrincipalImpl_nativeCreatePersistentForSigner(jenv *C.JNIEnv, jclass C.jclass, jSigner C.jobject, jDir C.jstring) C.jobject {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	signerObj := jutil.Object(uintptr(unsafe.Pointer(jSigner)))
	signer, err := GoSigner(env, signerObj)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	dir := jutil.GoString(env, jutil.Object(uintptr(unsafe.Pointer(jDir))))
	stateSerializer, err := vsecurity.NewPrincipalStateSerializer(dir)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	principal, err := vsecurity.NewPrincipalFromSigner(signer, stateSerializer)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	ref := jutil.GoNewRef(&principal) // Un-refed when the Java VPrincipalImpl is finalized.
	jPrincipal, err := jutil.NewObject(env, jVPrincipalImplClass, []jutil.Sign{jutil.LongSign, signerSign, blessingStoreSign, blessingRootsSign}, int64(ref), signerObj, jutil.NullObject, jutil.NullObject)
	if err != nil {
		jutil.GoDecRef(ref)
		jutil.JThrowV(env, err)
		return 0
	}
	return C.jobject(unsafe.Pointer(jPrincipal))
}

//export Java_io_v_v23_security_VPrincipalImpl_nativeBless
func Java_io_v_v23_security_VPrincipalImpl_nativeBless(jenv *C.JNIEnv, jVPrincipalImpl C.jobject, goRef C.jlong, jKey C.jobject, jWith C.jobject, jExtension C.jstring, jCaveat C.jobject, jAdditionalCaveats C.jobjectArray) C.jobject {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	key, err := GoPublicKey(env, jutil.Object(uintptr(unsafe.Pointer(jKey))))
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	with, err := GoBlessings(env, jutil.Object(uintptr(unsafe.Pointer(jWith))))
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	extension := jutil.GoString(env, jutil.Object(uintptr(unsafe.Pointer(jExtension))))
	caveat, err := GoCaveat(env, jutil.Object(uintptr(unsafe.Pointer(jCaveat))))
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	additionalCaveats, err := GoCaveats(env, jutil.Object(uintptr(unsafe.Pointer(jAdditionalCaveats))))
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	blessings, err := (*(*security.Principal)(jutil.GoRefValue(jutil.Ref(goRef)))).Bless(key, with, extension, caveat, additionalCaveats...)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	jBlessings, err := JavaBlessings(env, blessings)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	return C.jobject(unsafe.Pointer(jBlessings))
}

//export Java_io_v_v23_security_VPrincipalImpl_nativeBlessSelf
func Java_io_v_v23_security_VPrincipalImpl_nativeBlessSelf(jenv *C.JNIEnv, jVPrincipalImpl C.jobject, goRef C.jlong, jName C.jstring, jCaveats C.jobjectArray) C.jobject {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	name := jutil.GoString(env, jutil.Object(uintptr(unsafe.Pointer(jName))))
	caveats, err := GoCaveats(env, jutil.Object(uintptr(unsafe.Pointer(jCaveats))))
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	blessings, err := (*(*security.Principal)(jutil.GoRefValue(jutil.Ref(goRef)))).BlessSelf(name, caveats...)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	jBlessings, err := JavaBlessings(env, blessings)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	return C.jobject(unsafe.Pointer(jBlessings))
}

//export Java_io_v_v23_security_VPrincipalImpl_nativeSign
func Java_io_v_v23_security_VPrincipalImpl_nativeSign(jenv *C.JNIEnv, jVPrincipalImpl C.jobject, goRef C.jlong, jMessage C.jbyteArray) C.jobject {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	message := jutil.GoByteArray(env, jutil.Object(uintptr(unsafe.Pointer(jMessage))))
	sig, err := (*(*security.Principal)(jutil.GoRefValue(jutil.Ref(goRef)))).Sign(message)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	jSig, err := JavaSignature(env, sig)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	return C.jobject(unsafe.Pointer(jSig))
}

//export Java_io_v_v23_security_VPrincipalImpl_nativePublicKey
func Java_io_v_v23_security_VPrincipalImpl_nativePublicKey(jenv *C.JNIEnv, jVPrincipalImpl C.jobject, goRef C.jlong) C.jobject {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	key := (*(*security.Principal)(jutil.GoRefValue(jutil.Ref(goRef)))).PublicKey()
	jKey, err := JavaPublicKey(env, key)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	return C.jobject(unsafe.Pointer(jKey))
}

//export Java_io_v_v23_security_VPrincipalImpl_nativeBlessingStore
func Java_io_v_v23_security_VPrincipalImpl_nativeBlessingStore(jenv *C.JNIEnv, jVPrincipalImpl C.jobject, goRef C.jlong) C.jobject {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	store := (*(*security.Principal)(jutil.GoRefValue(jutil.Ref(goRef)))).BlessingStore()
	jStore, err := JavaBlessingStore(env, store)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	return C.jobject(unsafe.Pointer(jStore))
}

//export Java_io_v_v23_security_VPrincipalImpl_nativeRoots
func Java_io_v_v23_security_VPrincipalImpl_nativeRoots(jenv *C.JNIEnv, jVPrincipalImpl C.jobject, goRef C.jlong) C.jobject {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	roots := (*(*security.Principal)(jutil.GoRefValue(jutil.Ref(goRef)))).Roots()
	jRoots, err := JavaBlessingRoots(env, roots)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	return C.jobject(unsafe.Pointer(jRoots))
}

//export Java_io_v_v23_security_VPrincipalImpl_nativeFinalize
func Java_io_v_v23_security_VPrincipalImpl_nativeFinalize(jenv *C.JNIEnv, jVPrincipalImpl C.jobject, goRef C.jlong) {
	jutil.GoDecRef(jutil.Ref(goRef))
}

//export Java_io_v_v23_security_Blessings_nativeCreate
func Java_io_v_v23_security_Blessings_nativeCreate(jenv *C.JNIEnv, jBlessingsClass C.jclass, jWire C.jobject) C.jlong {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	var blessings security.Blessings
	if err := jutil.GoVomCopy(env, jutil.Object(uintptr(unsafe.Pointer(jWire))), jWireBlessingsClass, &blessings); err != nil {
		jutil.JThrowV(env, err)
		return C.jlong(0)
	}
	ref := jutil.GoNewRef(&blessings) // Un-refed when the Java Blessings object is finalized.
	return C.jlong(ref)
}

//export Java_io_v_v23_security_Blessings_nativeCreateUnion
func Java_io_v_v23_security_Blessings_nativeCreateUnion(jenv *C.JNIEnv, jBlessingsClass C.jclass, jBlessingsArr C.jobjectArray) C.jobject {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	blessingsArr, err := GoBlessingsArray(env, jutil.Object(uintptr(unsafe.Pointer(jBlessingsArr))))
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	blessings, err := security.UnionOfBlessings(blessingsArr...)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	jBlessings, err := JavaBlessings(env, blessings)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	return C.jobject(unsafe.Pointer(jBlessings))
}

//export Java_io_v_v23_security_Blessings_nativePublicKey
func Java_io_v_v23_security_Blessings_nativePublicKey(jenv *C.JNIEnv, jBlessings C.jobject, goRef C.jlong) C.jobject {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	key := (*(*security.Blessings)(jutil.GoRefValue(jutil.Ref(goRef)))).PublicKey()
	jPublicKey, err := JavaPublicKey(env, key)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	return C.jobject(unsafe.Pointer(jPublicKey))
}

//export Java_io_v_v23_security_Blessings_nativeSigningBlessings
func Java_io_v_v23_security_Blessings_nativeSigningBlessings(jenv *C.JNIEnv, jBlessings C.jobject, goRef C.jlong) C.jobject {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	blessings := security.SigningBlessings(*(*security.Blessings)(jutil.GoRefValue(jutil.Ref(goRef))))
	jSigningBlessings, err := JavaBlessings(env, blessings)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	return C.jobject(unsafe.Pointer(jSigningBlessings))
}

//export Java_io_v_v23_security_Blessings_nativeWireFormat
func Java_io_v_v23_security_Blessings_nativeWireFormat(jenv *C.JNIEnv, jBlessings C.jobject, goRef C.jlong) C.jobject {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	wire := security.MarshalBlessings(*(*security.Blessings)(jutil.GoRefValue(jutil.Ref(goRef))))
	jWire, err := JavaWireBlessings(env, wire)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	return C.jobject(unsafe.Pointer(jWire))
}

//export Java_io_v_v23_security_Blessings_nativeFinalize
func Java_io_v_v23_security_Blessings_nativeFinalize(jenv *C.JNIEnv, jBlessings C.jobject, goRef C.jlong) {
	jutil.GoDecRef(jutil.Ref(goRef))
}

//export Java_io_v_v23_security_BlessingRootsImpl_nativeAdd
func Java_io_v_v23_security_BlessingRootsImpl_nativeAdd(jenv *C.JNIEnv, jBlessingRootsImpl C.jobject, goRef C.jlong, jRoot C.jobject, jPattern C.jobject) {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	root, err := JavaPublicKeyToDER(env, jutil.Object(uintptr(unsafe.Pointer(jRoot))))
	if err != nil {
		jutil.JThrowV(env, err)
		return
	}
	pattern, err := GoBlessingPattern(env, jutil.Object(uintptr(unsafe.Pointer(jPattern))))
	if err != nil {
		jutil.JThrowV(env, err)
		return
	}
	if err := (*(*security.BlessingRoots)(jutil.GoRefValue(jutil.Ref(goRef)))).Add(root, pattern); err != nil {
		jutil.JThrowV(env, err)
		return
	}
}

//export Java_io_v_v23_security_BlessingRootsImpl_nativeRecognized
func Java_io_v_v23_security_BlessingRootsImpl_nativeRecognized(jenv *C.JNIEnv, jBlessingRootsImpl C.jobject, goRef C.jlong, jRoot C.jobject, jBlessing C.jstring) {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	root, err := JavaPublicKeyToDER(env, jutil.Object(uintptr(unsafe.Pointer(jRoot))))
	if err != nil {
		jutil.JThrowV(env, err)
		return
	}
	blessing := jutil.GoString(env, jutil.Object(uintptr(unsafe.Pointer(jBlessing))))
	if err := (*(*security.BlessingRoots)(jutil.GoRefValue(jutil.Ref(goRef)))).Recognized(root, blessing); err != nil {
		jutil.JThrowV(env, err)
	}
}

//export Java_io_v_v23_security_BlessingRootsImpl_nativeDebugString
func Java_io_v_v23_security_BlessingRootsImpl_nativeDebugString(jenv *C.JNIEnv, jBlessingRootsImpl C.jobject, goRef C.jlong) C.jstring {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	debug := (*(*security.BlessingRoots)(jutil.GoRefValue(jutil.Ref(goRef)))).DebugString()
	jDebug := jutil.JString(env, debug)
	return C.jstring(unsafe.Pointer(jDebug))
}

//export Java_io_v_v23_security_BlessingRootsImpl_nativeToString
func Java_io_v_v23_security_BlessingRootsImpl_nativeToString(jenv *C.JNIEnv, jBlessingRootsImpl C.jobject, goRef C.jlong) C.jstring {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	str := fmt.Sprintf("%v", (*(*security.BlessingRoots)(jutil.GoRefValue(jutil.Ref(goRef)))))
	jStr := jutil.JString(env, str)
	return C.jstring(unsafe.Pointer(jStr))
}

//export Java_io_v_v23_security_BlessingRootsImpl_nativeDump
func Java_io_v_v23_security_BlessingRootsImpl_nativeDump(jenv *C.JNIEnv, jBlessingRootsImpl C.jobject, goRef C.jlong) C.jobject {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	dump := (*(*security.BlessingRoots)(jutil.GoRefValue(jutil.Ref(goRef)))).Dump()
	result := make(map[jutil.Object][]jutil.Object)
	for pattern, keys := range dump {
		jBlessingPattern, err := JavaBlessingPattern(env, pattern)
		if err != nil {
			jutil.JThrowV(env, err)
			return 0
		}
		jPublicKeys := make([]jutil.Object, len(keys))
		for i, key := range keys {
			var err error
			if jPublicKeys[i], err = JavaPublicKey(env, key); err != nil {
				jutil.JThrowV(env, err)
				return 0
			}
		}
		result[jBlessingPattern] = jPublicKeys
	}
	jMap, err := jutil.JObjectMultimap(env, result)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	return C.jobject(unsafe.Pointer(jMap))
}

//export Java_io_v_v23_security_BlessingRootsImpl_nativeFinalize
func Java_io_v_v23_security_BlessingRootsImpl_nativeFinalize(jenv *C.JNIEnv, jBlessingRootsImpl C.jobject, goRef C.jlong) {
	jutil.GoDecRef(jutil.Ref(goRef))
}

//export Java_io_v_v23_security_BlessingStoreImpl_nativeSet
func Java_io_v_v23_security_BlessingStoreImpl_nativeSet(jenv *C.JNIEnv, jBlessingStoreImpl C.jobject, goRef C.jlong, jBlessings C.jobject, jForPeers C.jobject) C.jobject {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	blessings, err := GoBlessings(env, jutil.Object(uintptr(unsafe.Pointer(jBlessings))))
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	forPeers, err := GoBlessingPattern(env, jutil.Object(uintptr(unsafe.Pointer(jForPeers))))
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	oldBlessings, err := (*(*security.BlessingStore)(jutil.GoRefValue(jutil.Ref(goRef)))).Set(blessings, forPeers)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	jOldBlessings, err := JavaBlessings(env, oldBlessings)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	return C.jobject(unsafe.Pointer(jOldBlessings))
}

//export Java_io_v_v23_security_BlessingStoreImpl_nativeForPeer
func Java_io_v_v23_security_BlessingStoreImpl_nativeForPeer(jenv *C.JNIEnv, jBlessingStoreImpl C.jobject, goRef C.jlong, jPeerBlessings C.jobjectArray) C.jobject {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	peerBlessings, err := jutil.GoStringArray(env, jutil.Object(uintptr(unsafe.Pointer(jPeerBlessings))))
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	blessings := (*(*security.BlessingStore)(jutil.GoRefValue(jutil.Ref(goRef)))).ForPeer(peerBlessings...)
	jBlessings, err := JavaBlessings(env, blessings)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	return C.jobject(unsafe.Pointer(jBlessings))
}

//export Java_io_v_v23_security_BlessingStoreImpl_nativeSetDefaultBlessings
func Java_io_v_v23_security_BlessingStoreImpl_nativeSetDefaultBlessings(jenv *C.JNIEnv, jBlessingStoreImpl C.jobject, goRef C.jlong, jBlessings C.jobject) {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	blessings, err := GoBlessings(env, jutil.Object(uintptr(unsafe.Pointer(jBlessings))))
	if err != nil {
		jutil.JThrowV(env, err)
		return
	}
	if err := (*(*security.BlessingStore)(jutil.GoRefValue(jutil.Ref(goRef)))).SetDefault(blessings); err != nil {
		jutil.JThrowV(env, err)
	}
}

//export Java_io_v_v23_security_BlessingStoreImpl_nativeDefaultBlessings
func Java_io_v_v23_security_BlessingStoreImpl_nativeDefaultBlessings(jenv *C.JNIEnv, jBlessingStoreImpl C.jobject, goRef C.jlong) C.jobject {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	blessings, _ := (*(*security.BlessingStore)(jutil.GoRefValue(jutil.Ref(goRef)))).Default()
	jBlessings, err := JavaBlessings(env, blessings)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	return C.jobject(unsafe.Pointer(jBlessings))
}

//export Java_io_v_v23_security_BlessingStoreImpl_nativePublicKey
func Java_io_v_v23_security_BlessingStoreImpl_nativePublicKey(jenv *C.JNIEnv, jBlessingStoreImpl C.jobject, goRef C.jlong) C.jobject {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	key := (*(*security.BlessingStore)(jutil.GoRefValue(jutil.Ref(goRef)))).PublicKey()
	jKey, err := JavaPublicKey(env, key)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	return C.jobject(unsafe.Pointer(jKey))
}

//export Java_io_v_v23_security_BlessingStoreImpl_nativePeerBlessings
func Java_io_v_v23_security_BlessingStoreImpl_nativePeerBlessings(jenv *C.JNIEnv, jBlessingStoreImpl C.jobject, goRef C.jlong) C.jobject {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	blessingsMap := (*(*security.BlessingStore)(jutil.GoRefValue(jutil.Ref(goRef)))).PeerBlessings()
	bmap := make(map[jutil.Object]jutil.Object)
	for pattern, blessings := range blessingsMap {
		jPattern, err := JavaBlessingPattern(env, pattern)
		if err != nil {
			jutil.JThrowV(env, err)
			return 0
		}
		jBlessings, err := JavaBlessings(env, blessings)
		if err != nil {
			jutil.JThrowV(env, err)
			return 0
		}
		bmap[jPattern] = jBlessings
	}
	jBlessingsMap, err := jutil.JObjectMap(env, bmap)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	return C.jobject(unsafe.Pointer(jBlessingsMap))
}

//export Java_io_v_v23_security_BlessingStoreImpl_nativeCacheDischarge
func Java_io_v_v23_security_BlessingStoreImpl_nativeCacheDischarge(jenv *C.JNIEnv, jBlessingStoreImpl C.jobject, goRef C.jlong, jDischarge C.jobject, jCaveat C.jobject, jImpetus C.jobject) {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	blessingStore := *(*security.BlessingStore)(jutil.GoRefValue(jutil.Ref(goRef)))
	discharge, err := GoDischarge(env, jutil.Object(uintptr(unsafe.Pointer(jDischarge))))
	if err != nil {
		jutil.JThrowV(env, err)
		return
	}
	caveat, err := GoCaveat(env, jutil.Object(uintptr(unsafe.Pointer(jCaveat))))
	if err != nil {
		jutil.JThrowV(env, err)
		return
	}
	var impetus security.DischargeImpetus
	err = jutil.GoVomCopy(env, jutil.Object(uintptr(unsafe.Pointer(jImpetus))), jDischargeImpetusClass, &impetus)
	if err != nil {
		jutil.JThrowV(env, err)
		return
	}
	blessingStore.CacheDischarge(discharge, caveat, impetus)
}

//export Java_io_v_v23_security_BlessingStoreImpl_nativeClearDischarges
func Java_io_v_v23_security_BlessingStoreImpl_nativeClearDischarges(jenv *C.JNIEnv, jBlessingStoreImpl C.jobject, goRef C.jlong, jDischarges C.jobject) {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	blessingStore := *(*security.BlessingStore)(jutil.GoRefValue(jutil.Ref(goRef)))
	arr, err := jutil.GoObjectArray(env, jutil.Object(uintptr(unsafe.Pointer(jDischarges))))
	if err != nil {
		jutil.JThrowV(env, err)
		return
	}
	var discharges []security.Discharge
	for _, jDischarge := range arr {
		discharge, err := GoDischarge(env, jDischarge)
		if err != nil {
			jutil.JThrowV(env, err)
			return
		}
		discharges = append(discharges, discharge)
	}
	blessingStore.ClearDischarges(discharges...)
}

//export Java_io_v_v23_security_BlessingStoreImpl_nativeDischarge
func Java_io_v_v23_security_BlessingStoreImpl_nativeDischarge(jenv *C.JNIEnv, jBlessingStoreImpl C.jobject, goRef C.jlong, jCaveat C.jobject, jImpetus C.jobject) C.jobject {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	blessingStore := *(*security.BlessingStore)(jutil.GoRefValue(jutil.Ref(goRef)))
	caveat, err := GoCaveat(env, jutil.Object(uintptr(unsafe.Pointer(jCaveat))))
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	var impetus security.DischargeImpetus
	err = jutil.GoVomCopy(env, jutil.Object(uintptr(unsafe.Pointer(jImpetus))), jDischargeImpetusClass, &impetus)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	// TODO(sjr): support cachedTime in Java
	discharge, _ := blessingStore.Discharge(caveat, impetus)
	jDischarge, err := JavaDischarge(env, discharge)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	return C.jobject(unsafe.Pointer(jDischarge))
}

//export Java_io_v_v23_security_BlessingStoreImpl_nativeDebugString
func Java_io_v_v23_security_BlessingStoreImpl_nativeDebugString(jenv *C.JNIEnv, jBlessingStoreImpl C.jobject, goRef C.jlong) C.jstring {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	debug := (*(*security.BlessingStore)(jutil.GoRefValue(jutil.Ref(goRef)))).DebugString()
	jDebug := jutil.JString(env, debug)
	return C.jstring(unsafe.Pointer(jDebug))
}

//export Java_io_v_v23_security_BlessingStoreImpl_nativeToString
func Java_io_v_v23_security_BlessingStoreImpl_nativeToString(jenv *C.JNIEnv, jBlessingStoreImpl C.jobject, goRef C.jlong) C.jstring {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	str := fmt.Sprintf("%s", (*(*security.BlessingStore)(jutil.GoRefValue(jutil.Ref(goRef)))))
	jStr := jutil.JString(env, str)
	return C.jstring(unsafe.Pointer(jStr))
}

//export Java_io_v_v23_security_BlessingStoreImpl_nativeFinalize
func Java_io_v_v23_security_BlessingStoreImpl_nativeFinalize(jenv *C.JNIEnv, jBlessingStoreImpl C.jobject, goRef C.jlong) {
	jutil.GoDecRef(jutil.Ref(goRef))
}

//export Java_io_v_v23_security_BlessingPattern_nativeCreate
func Java_io_v_v23_security_BlessingPattern_nativeCreate(jenv *C.JNIEnv, jBlessingPatternClass C.jclass, jValue C.jstring) C.jlong {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	pattern := security.BlessingPattern(jutil.GoString(env, jutil.Object(uintptr(unsafe.Pointer(jValue)))))
	ref := jutil.GoNewRef(&pattern) // Un-refed when the BlessingPattern object is finalized.
	return C.jlong(ref)
}

//export Java_io_v_v23_security_BlessingPattern_nativeIsMatchedBy
func Java_io_v_v23_security_BlessingPattern_nativeIsMatchedBy(jenv *C.JNIEnv, jBlessingPattern C.jobject, goRef C.jlong, jBlessings C.jobjectArray) C.jboolean {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	blessings, err := jutil.GoStringArray(env, jutil.Object(uintptr(unsafe.Pointer(jBlessings))))
	if err != nil {
		jutil.JThrowV(env, err)
		return C.JNI_FALSE
	}

	matched := (*(*security.BlessingPattern)(jutil.GoRefValue(jutil.Ref(goRef)))).MatchedBy(blessings...)
	if matched {
		return C.JNI_TRUE
	}
	return C.JNI_FALSE
}

//export Java_io_v_v23_security_BlessingPattern_nativeIsValid
func Java_io_v_v23_security_BlessingPattern_nativeIsValid(jenv *C.JNIEnv, jBlessingPattern C.jobject, goRef C.jlong) C.jboolean {
	valid := (*(*security.BlessingPattern)(jutil.GoRefValue(jutil.Ref(goRef)))).IsValid()
	if valid {
		return C.JNI_TRUE
	}
	return C.JNI_FALSE
}

//export Java_io_v_v23_security_BlessingPattern_nativeMakeNonExtendable
func Java_io_v_v23_security_BlessingPattern_nativeMakeNonExtendable(jenv *C.JNIEnv, jBlessingPattern C.jobject, goRef C.jlong) C.jobject {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	pattern := (*(*security.BlessingPattern)(jutil.GoRefValue(jutil.Ref(goRef)))).MakeNonExtendable()
	jPattern, err := JavaBlessingPattern(env, pattern)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	return C.jobject(unsafe.Pointer(jPattern))
}

//export Java_io_v_v23_security_BlessingPattern_nativeFinalize
func Java_io_v_v23_security_BlessingPattern_nativeFinalize(jenv *C.JNIEnv, jBlessingPattern C.jobject, goRef C.jlong) {
	jutil.GoDecRef(jutil.Ref(goRef))
}

//export Java_io_v_v23_security_PublicKeyThirdPartyCaveatValidator_nativeValidate
func Java_io_v_v23_security_PublicKeyThirdPartyCaveatValidator_nativeValidate(jenv *C.JNIEnv, jThirdPartyValidatorClass C.jclass, jContext C.jobject, jCall C.jobject, jCaveatParam C.jobject) {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	param, err := jutil.GoVomCopyValue(env, jutil.Object(uintptr(unsafe.Pointer(jCaveatParam))))
	if err != nil {
		jutil.JThrowV(env, err)
		return
	}
	ctx, _, err := jcontext.GoContext(env, jutil.Object(uintptr(unsafe.Pointer(jContext))))
	if err != nil {
		jutil.JThrowV(env, err)
		return
	}
	call, err := GoCall(env, jutil.Object(uintptr(unsafe.Pointer(jCall))))
	if err != nil {
		jutil.JThrowV(env, err)
		return
	}
	caveat, err := security.NewCaveat(security.PublicKeyThirdPartyCaveat, param)
	if err != nil {
		jutil.JThrowV(env, err)
		return
	}
	if err := caveat.Validate(ctx, call); err != nil {
		jutil.JThrowV(env, err)
		return
	}
}

//export Java_io_v_v23_security_VSecurity_nativeGetRemoteBlessingNames
func Java_io_v_v23_security_VSecurity_nativeGetRemoteBlessingNames(jenv *C.JNIEnv, jVSecurityClass C.jclass, jCtx C.jobject, jCall C.jobject) C.jobjectArray {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	ctx, _, err := jcontext.GoContext(env, jutil.Object(uintptr(unsafe.Pointer(jCtx))))
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	call, err := GoCall(env, jutil.Object(uintptr(unsafe.Pointer(jCall))))
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	blessingStrs, _ := security.RemoteBlessingNames(ctx, call)
	jArr, err := jutil.JStringArray(env, blessingStrs)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	return C.jobjectArray(unsafe.Pointer(jArr))
}

//export Java_io_v_v23_security_VSecurity_nativeGetSigningBlessingNames
func Java_io_v_v23_security_VSecurity_nativeGetSigningBlessingNames(jenv *C.JNIEnv, jVSecurityClass C.jclass, jCtx C.jobject, jPrincipal C.jobject, jBlessings C.jobject) C.jobjectArray {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	ctx, _, err := jcontext.GoContext(env, jutil.Object(uintptr(unsafe.Pointer(jCtx))))
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	principal, err := GoPrincipal(env, jutil.Object(uintptr(unsafe.Pointer(jPrincipal))))
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	blessings, err := GoBlessings(env, jutil.Object(uintptr(unsafe.Pointer(jBlessings))))
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	blessingStrs, _ := security.SigningBlessingNames(ctx, principal, blessings)
	jArr, err := jutil.JStringArray(env, blessingStrs)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	return C.jobjectArray(unsafe.Pointer(jArr))
}

//export Java_io_v_v23_security_VSecurity_nativeGetLocalBlessingNames
func Java_io_v_v23_security_VSecurity_nativeGetLocalBlessingNames(jenv *C.JNIEnv, jVSecurityClass C.jclass, jCtx C.jobject, jCall C.jobject) C.jobjectArray {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	ctx, _, err := jcontext.GoContext(env, jutil.Object(uintptr(unsafe.Pointer(jCtx))))
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	call, err := GoCall(env, jutil.Object(uintptr(unsafe.Pointer(jCall))))
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	blessingStrs := security.LocalBlessingNames(ctx, call)
	jArr, err := jutil.JStringArray(env, blessingStrs)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	return C.jobjectArray(unsafe.Pointer(jArr))
}

//export Java_io_v_v23_security_VSecurity_nativeGetBlessingNames
func Java_io_v_v23_security_VSecurity_nativeGetBlessingNames(jenv *C.JNIEnv, jVSecurityClass C.jclass, jPrincipal C.jobject, jBlessings C.jobject) C.jobjectArray {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	principal, err := GoPrincipal(env, jutil.Object(uintptr(unsafe.Pointer(jPrincipal))))
	if err != nil {
		jutil.JThrowV(env, err)
	}
	blessings, err := GoBlessings(env, jutil.Object(uintptr(unsafe.Pointer(jBlessings))))
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	blessingStrs := security.BlessingNames(principal, blessings)
	jArr, err := jutil.JStringArray(env, blessingStrs)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	return C.jobjectArray(unsafe.Pointer(jArr))
}

//export Java_io_v_v23_security_VSecurity_nativeAddToRoots
func Java_io_v_v23_security_VSecurity_nativeAddToRoots(jenv *C.JNIEnv, jVSecurityClass C.jclass, jPrincipal C.jobject, jBlessings C.jobject) {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	principal, err := GoPrincipal(env, jutil.Object(uintptr(unsafe.Pointer(jPrincipal))))
	if err != nil {
		jutil.JThrowV(env, err)
	}
	blessings, err := GoBlessings(env, jutil.Object(uintptr(unsafe.Pointer(jBlessings))))
	if err != nil {
		jutil.JThrowV(env, err)
		return
	}
	if err := security.AddToRoots(principal, blessings); err != nil {
		jutil.JThrowV(env, err)
	}
}

//export Java_io_v_v23_security_VSecurity_nativeCreateAuthorizer
func Java_io_v_v23_security_VSecurity_nativeCreateAuthorizer(jenv *C.JNIEnv, jVSecurityClass C.jclass, kind C.jint, jKey C.jobject) C.jobject {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	var auth security.Authorizer
	switch kind {
	case 0:
		auth = security.AllowEveryone()
	case 1:
		auth = security.EndpointAuthorizer()
	case 2:
		auth = security.DefaultAuthorizer()
	case 3:
		key, err := GoPublicKey(env, jutil.Object(uintptr(unsafe.Pointer(jKey))))
		if err != nil {
			jutil.JThrowV(env, err)
			return 0
		}
		auth = security.PublicKeyAuthorizer(key)
	default:
		return 0
	}
	ref := jutil.GoNewRef(&auth) // Un-refed when the Java PermissionsAuthorizer is finalized
	jAuthorizer, err := jutil.NewObject(env, jutil.Class(uintptr(unsafe.Pointer(jPermissionsAuthorizerClass))), []jutil.Sign{jutil.LongSign}, int64(ref))
	if err != nil {
		jutil.GoDecRef(ref)
		jutil.JThrowV(env, err)
		return 0
	}
	return C.jobject(unsafe.Pointer(jAuthorizer))
}
