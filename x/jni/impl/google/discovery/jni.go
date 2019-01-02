// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build java android

package discovery

import (
	"unsafe"

	"v.io/v23/discovery"
	"v.io/v23/security"
	"v.io/v23/verror"

	idiscovery "v.io/x/ref/lib/discovery"
	gdiscovery "v.io/x/ref/lib/discovery/global"

	jchannel "v.io/x/jni/impl/google/channel"
	jutil "v.io/x/jni/util"
	jcontext "v.io/x/jni/v23/context"
)

// #include "jni.h"
// #include <stdlib.h>
import "C"

var (
	adIdSign          = jutil.ClassSign("io.v.v23.discovery.AdId")
	advertisementSign = jutil.ClassSign("io.v.v23.discovery.Advertisement")

	jAdIdClass            jutil.Class // io.v.v23.discovery.AdId
	jAdvertisementClass   jutil.Class // io.v.v23.discovery.Advertisement
	jBlessingPatternClass jutil.Class // io.v.v23.security.BlessingPattern
	jDiscoveryImplClass   jutil.Class // io.v.impl.google.lib.discovery.DiscoveryImpl
	jUpdateImplClass      jutil.Class // io.v.impl.google.lib.discovery.UpdateImpl
	jUUIDClass            jutil.Class // java.util.UUID
)

// Init initializes the JNI code with the given Java environment. This method
// must be called from the main Java thread.
func Init(env jutil.Env) error {
	// Cache global references to all Java classes used by the package.  This is
	// necessary because JNI gets access to the class loader only in the system
	// thread, so we aren't able to invoke FindClass in other threads.
	var err error
	jAdIdClass, err = jutil.JFindClass(env, "io/v/v23/discovery/AdId")
	if err != nil {
		return err
	}
	jAdvertisementClass, err = jutil.JFindClass(env, "io/v/v23/discovery/Advertisement")
	if err != nil {
		return err
	}
	jBlessingPatternClass, err = jutil.JFindClass(env, "io/v/v23/security/BlessingPattern")
	if err != nil {
		return err
	}
	jDiscoveryImplClass, err = jutil.JFindClass(env, "io/v/impl/google/lib/discovery/DiscoveryImpl")
	if err != nil {
		return err
	}
	jUpdateImplClass, err = jutil.JFindClass(env, "io/v/impl/google/lib/discovery/UpdateImpl")
	if err != nil {
		return err
	}
	jUUIDClass, err = jutil.JFindClass(env, "java/util/UUID")
	if err != nil {
		return err
	}
	return nil
}

//export Java_io_v_impl_google_lib_discovery_UUIDUtil_serviceUUID
func Java_io_v_impl_google_lib_discovery_UUIDUtil_serviceUUID(jenv *C.JNIEnv, _ C.jclass, jName C.jstring) C.jobject {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	name := jutil.GoString(env, jutil.Object(uintptr(unsafe.Pointer(jName))))
	jUuid, err := javaUUID(env, idiscovery.NewServiceUUID(name))
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	return C.jobject(unsafe.Pointer(jUuid))
}

//export Java_io_v_impl_google_lib_discovery_UUIDUtil_attributeUUID
func Java_io_v_impl_google_lib_discovery_UUIDUtil_attributeUUID(jenv *C.JNIEnv, _ C.jclass, jName C.jstring) C.jobject {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	name := jutil.GoString(env, jutil.Object(uintptr(unsafe.Pointer(jName))))
	jUuid, err := javaUUID(env, idiscovery.NewAttributeUUID(name))
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	return C.jobject(unsafe.Pointer(jUuid))
}

//export Java_io_v_impl_google_lib_discovery_DiscoveryImpl_nativeAdvertise
func Java_io_v_impl_google_lib_discovery_DiscoveryImpl_nativeAdvertise(jenv *C.JNIEnv, _ C.jobject, goRef C.jlong, jCtx C.jobject, jAdObj C.jobject, jVisibilityObj C.jobject, jCbObj C.jobject) {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	ctx, _, err := jcontext.GoContext(env, jutil.Object(uintptr(unsafe.Pointer(jCtx))))
	if err != nil {
		jutil.JThrowV(env, err)
		return
	}

	d := *(*discovery.T)(jutil.GoRefValue(jutil.Ref(goRef)))
	jAd := jutil.Object(uintptr(unsafe.Pointer(jAdObj)))
	jVisibility := jutil.Object(uintptr(unsafe.Pointer(jVisibilityObj)))

	var ad discovery.Advertisement
	if err := jutil.GoVomCopy(env, jAd, jAdvertisementClass, &ad); err != nil {
		jutil.JThrowV(env, err)
		return
	}

	jVisibilityList, err := jutil.GoObjectList(env, jVisibility)
	if err != nil {
		jutil.JThrowV(env, err)
		return
	}
	visibility := make([]security.BlessingPattern, len(jVisibilityList))
	for i, jPattern := range jVisibilityList {
		if err := jutil.GoVomCopy(env, jPattern, jBlessingPatternClass, &visibility[i]); err != nil {
			jutil.JThrowV(env, err)
			return
		}
	}

	done, err := d.Advertise(ctx, &ad, visibility)
	if err != nil {
		jutil.JThrowV(env, err)
		return
	}

	// Copy back the advertisement id.
	jId, err := jutil.JVomCopy(env, &ad.Id, jAdIdClass)
	if err != nil {
		jutil.JThrowV(env, err)
		return
	}
	if err = jutil.CallVoidMethod(env, jAd, "setId", []jutil.Sign{adIdSign}, jId); err != nil {
		jutil.JThrowV(env, err)
		return
	}

	jCb := jutil.Object(uintptr(unsafe.Pointer(jCbObj)))
	jutil.DoAsyncCall(env, jCb, func() (jutil.Object, error) {
		<-done
		return jutil.NullObject, nil
	})
}

//export Java_io_v_impl_google_lib_discovery_DiscoveryImpl_nativeScan
func Java_io_v_impl_google_lib_discovery_DiscoveryImpl_nativeScan(jenv *C.JNIEnv, _ C.jobject, goRef C.jlong, jCtx C.jobject, jQuery C.jstring) C.jobject {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	ctx, _, err := jcontext.GoContext(env, jutil.Object(uintptr(unsafe.Pointer(jCtx))))
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}

	d := *(*discovery.T)(jutil.GoRefValue(jutil.Ref(goRef)))
	query := jutil.GoString(env, jutil.Object(uintptr(unsafe.Pointer(jQuery))))

	scanCh, err := d.Scan(ctx, query)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}

	jChannel, err := jchannel.JavaInputChannel(env, ctx, nil, func() (jutil.Object, error) {
		update, ok := <-scanCh
		if !ok {
			return jutil.NullObject, verror.NewErrEndOfFile(ctx)
		}

		env, freeFunc := jutil.GetEnv()
		defer freeFunc()

		jUpdate, err := javaUpdate(env, update)
		if err != nil {
			return jutil.NullObject, err
		}
		// Must grab a global reference as we free up the env and all local references that come
		// along with it.
		return jutil.NewGlobalRef(env, jUpdate), nil // un-refed by InputChannelImpl_nativeRecv
	})
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	return C.jobject(unsafe.Pointer(jChannel))
}

//export Java_io_v_impl_google_lib_discovery_DiscoveryImpl_nativeFinalize
func Java_io_v_impl_google_lib_discovery_DiscoveryImpl_nativeFinalize(jenv *C.JNIEnv, _ C.jobject, goRef C.jlong) {
	jutil.GoDecRef(jutil.Ref(goRef))
}

//export Java_io_v_impl_google_lib_discovery_UpdateImpl_nativeAttachment
func Java_io_v_impl_google_lib_discovery_UpdateImpl_nativeAttachment(jenv *C.JNIEnv, _ C.jobject, goRef C.jlong, jCtx C.jobject, jName C.jstring, jCbObj C.jobject) {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	ctx, _, err := jcontext.GoContext(env, jutil.Object(uintptr(unsafe.Pointer(jCtx))))
	if err != nil {
		jutil.JThrowV(env, err)
		return
	}

	update := *(*discovery.Update)(jutil.GoRefValue(jutil.Ref(goRef)))
	name := jutil.GoString(env, jutil.Object(uintptr(unsafe.Pointer(jName))))

	jCb := jutil.Object(uintptr(unsafe.Pointer(jCbObj)))
	jutil.DoAsyncCall(env, jCb, func() (jutil.Object, error) {
		dataOrErr := <-update.Attachment(ctx, name)
		if dataOrErr.Error != nil {
			return jutil.NullObject, err
		}

		env, freeFunc := jutil.GetEnv()
		defer freeFunc()

		jData, err := jutil.JByteArray(env, dataOrErr.Data)
		if err != nil {
			return jutil.NullObject, err
		}
		// Must grab a global reference as we free up the env and all local references that come
		// along with it.
		return jutil.NewGlobalRef(env, jData), nil // Un-refed in DoAsyncCall
	})
}

//export Java_io_v_impl_google_lib_discovery_UpdateImpl_nativeFinalize
func Java_io_v_impl_google_lib_discovery_UpdateImpl_nativeFinalize(jenv *C.JNIEnv, _ C.jobject, goRef C.jlong) {
	jutil.GoDecRef(jutil.Ref(goRef))
}

//export Java_io_v_impl_google_lib_discovery_GlobalDiscovery_nativeNewDiscovery
func Java_io_v_impl_google_lib_discovery_GlobalDiscovery_nativeNewDiscovery(jenv *C.JNIEnv, jRuntime C.jclass, jContext C.jobject, jPath C.jstring, jMountTTL, jScanInterval C.jobject) C.jobject {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	ctx, _, err := jcontext.GoContext(env, jutil.Object(uintptr(unsafe.Pointer(jContext))))
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	path := jutil.GoString(env, jutil.Object(uintptr(unsafe.Pointer(jPath))))
	mountTTL, err := jutil.GoDuration(env, jutil.Object(uintptr(unsafe.Pointer(jMountTTL))))
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	scanInterval, err := jutil.GoDuration(env, jutil.Object(uintptr(unsafe.Pointer(jScanInterval))))
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}

	discovery, err := gdiscovery.NewWithTTL(ctx, path, mountTTL, scanInterval)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	jDiscovery, err := JavaDiscovery(env, discovery)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	return C.jobject(unsafe.Pointer(jDiscovery))
}

//export Java_io_v_impl_google_lib_discovery_FactoryUtil_injectMockPlugin
func Java_io_v_impl_google_lib_discovery_FactoryUtil_injectMockPlugin(jenv *C.JNIEnv, _ C.jclass, jCtx C.jobject) {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	ctx, _, err := jcontext.GoContext(env, jutil.Object(uintptr(unsafe.Pointer(jCtx))))
	if err != nil {
		jutil.JThrowV(env, err)
		return
	}
	injectMockPlugin(ctx)
}
