// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build java android

package namespace

import (
	"time"
	"unsafe"

	"v.io/v23/context"
	"v.io/v23/namespace"
	"v.io/v23/naming"
	"v.io/v23/security/access"
	"v.io/v23/verror"

	jchannel "v.io/x/jni/impl/google/channel"
	jutil "v.io/x/jni/util"
	jcontext "v.io/x/jni/v23/context"
)

// #include "jni.h"
import "C"

var (
	// Global reference for io.v.impl.google.namespace.NamespaceImpl class.
	jNamespaceImplClass jutil.Class
	// Global reference for io.v.v23.naming.GlobReply class.
	jGlobReplyClass jutil.Class
	// Global reference for io.v.v23.naming.MountEntry class.
	JMountEntryClass jutil.Class
	// Global reference for io.v.v23.security.access.Permissions
	jPermissionsClass jutil.Class
)

// Init initializes the JNI code with the given Java environment. This method
// must be called from the main Java thread.
func Init(env jutil.Env) error {
	var err error
	jNamespaceImplClass, err = jutil.JFindClass(env, "io/v/impl/google/namespace/NamespaceImpl")
	if err != nil {
		return err
	}
	jGlobReplyClass, err = jutil.JFindClass(env, "io/v/v23/naming/GlobReply")
	if err != nil {
		return err
	}
	JMountEntryClass, err = jutil.JFindClass(env, "io/v/v23/naming/MountEntry")
	if err != nil {
		return err
	}
	jPermissionsClass, err = jutil.JFindClass(env, "io/v/v23/security/access/Permissions")
	if err != nil {
		return err
	}
	return nil
}

func globArgs(env jutil.Env, jContext C.jobject, jPattern C.jstring, jOptions C.jobject) (context *context.T, cancel func(), pattern string, opts []naming.NamespaceOpt, err error) {
	context, cancel, err = jcontext.GoContext(env, jutil.Object(uintptr(unsafe.Pointer(jContext))))
	if err != nil {
		return
	}
	opts, err = goNamespaceOptions(env, jutil.Object(uintptr(unsafe.Pointer(jOptions))))
	if err != nil {
		return
	}
	pattern = jutil.GoString(env, jutil.Object(uintptr(unsafe.Pointer(jPattern))))
	return
}

//export Java_io_v_impl_google_namespace_NamespaceImpl_nativeGlob
func Java_io_v_impl_google_namespace_NamespaceImpl_nativeGlob(jenv *C.JNIEnv, jNamespaceClass C.jclass, goRef C.jlong, jContext C.jobject, jPattern C.jstring, jOptions C.jobject) C.jobject {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	n := *(*namespace.T)(jutil.GoRefValue(jutil.Ref(goRef)))
	ctx, cancel, pattern, opts, err := globArgs(env, jContext, jPattern, jOptions)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	var globChannel <-chan naming.GlobReply
	var globError error
	globDone := make(chan bool)
	go func() {
		globChannel, globError = n.Glob(ctx, pattern, opts...)
		close(globDone)
	}()
	jChannel, err := jchannel.JavaInputChannel(env, ctx, cancel, func() (jutil.Object, error) {
		// A few blocking calls below - don't call GetEnv() before they complete.
		<-globDone
		if globError != nil {
			return jutil.NullObject, globError
		}
		globReply, ok := <-globChannel
		if !ok {
			return jutil.NullObject, verror.NewErrEndOfFile(ctx)
		}
		env, freeFunc := jutil.GetEnv()
		defer freeFunc()
		jReply, err := jutil.JVomCopy(env, globReply, jGlobReplyClass)
		if err != nil {
			return jutil.NullObject, err
		}
		// Must grab a global reference as we free up the env and all local references that come
		// along with it.
		return jutil.NewGlobalRef(env, jReply), nil // Un-refed by InputChannelImpl_nativeRecv
	})
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	return C.jobject(unsafe.Pointer(jChannel))
}

func mountArgs(env jutil.Env, jContext C.jobject, jName, jServer C.jstring, jDuration, jOptions C.jobject) (context *context.T, name, server string, duration time.Duration, options []naming.NamespaceOpt, err error) {
	context, _, err = jcontext.GoContext(env, jutil.Object(uintptr(unsafe.Pointer(jContext))))
	if err != nil {
		return
	}
	name = jutil.GoString(env, jutil.Object(uintptr(unsafe.Pointer(jName))))
	server = jutil.GoString(env, jutil.Object(uintptr(unsafe.Pointer(jServer))))
	duration, err = jutil.GoDuration(env, jutil.Object(uintptr(unsafe.Pointer(jDuration))))
	if err != nil {
		return
	}
	options, err = goNamespaceOptions(env, jutil.Object(uintptr(unsafe.Pointer(jOptions))))
	return
}

//export Java_io_v_impl_google_namespace_NamespaceImpl_nativeMount
func Java_io_v_impl_google_namespace_NamespaceImpl_nativeMount(jenv *C.JNIEnv, jNamespaceClass C.jclass, goRef C.jlong, jContext C.jobject, jName C.jstring, jServer C.jstring, jDuration C.jobject, jOptions C.jobject, jCallbackObj C.jobject) {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	n := *(*namespace.T)(jutil.GoRefValue(jutil.Ref(goRef)))
	jCallback := jutil.Object(uintptr(unsafe.Pointer(jCallbackObj)))
	context, name, server, duration, options, err := mountArgs(env, jContext, jName, jServer, jDuration, jOptions)
	if err != nil {
		jutil.CallbackOnFailure(env, jCallback, err)
		return
	}
	jutil.DoAsyncCall(env, jCallback, func() (jutil.Object, error) {
		return jutil.NullObject, n.Mount(context, name, server, duration, options...)
	})
}

func unmountArgs(env jutil.Env, jName, jServer C.jstring, jContext, jOptions C.jobject) (name, server string, context *context.T, options []naming.NamespaceOpt, err error) {
	name = jutil.GoString(env, jutil.Object(uintptr(unsafe.Pointer(jName))))
	server = jutil.GoString(env, jutil.Object(uintptr(unsafe.Pointer(jServer))))
	context, _, err = jcontext.GoContext(env, jutil.Object(uintptr(unsafe.Pointer(jContext))))
	if err != nil {
		return
	}
	options, err = goNamespaceOptions(env, jutil.Object(uintptr(unsafe.Pointer(jOptions))))
	return
}

//export Java_io_v_impl_google_namespace_NamespaceImpl_nativeUnmount
func Java_io_v_impl_google_namespace_NamespaceImpl_nativeUnmount(jenv *C.JNIEnv, jNamespaceClass C.jclass, goRef C.jlong, jContext C.jobject, jName C.jstring, jServer C.jstring, jOptions C.jobject, jCallbackObj C.jobject) {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	n := *(*namespace.T)(jutil.GoRefValue(jutil.Ref(goRef)))
	jCallback := jutil.Object(uintptr(unsafe.Pointer(jCallbackObj)))
	name, server, context, options, err := unmountArgs(env, jName, jServer, jContext, jOptions)
	if err != nil {
		jutil.CallbackOnFailure(env, jCallback, err)
		return
	}
	jutil.DoAsyncCall(env, jCallback, func() (jutil.Object, error) {
		return jutil.NullObject, n.Unmount(context, name, server, options...)
	})
}

func deleteArgs(env jutil.Env, jContext, jOptions C.jobject, jName C.jstring, jDeleteSubtree C.jboolean) (context *context.T, options []naming.NamespaceOpt, name string, deleteSubtree bool, err error) {
	context, _, err = jcontext.GoContext(env, jutil.Object(uintptr(unsafe.Pointer(jContext))))
	if err != nil {
		return
	}
	options, err = goNamespaceOptions(env, jutil.Object(uintptr(unsafe.Pointer(jOptions))))
	if err != nil {
		return
	}
	name = jutil.GoString(env, jutil.Object(uintptr(unsafe.Pointer(jName))))
	deleteSubtree = jDeleteSubtree == C.JNI_TRUE
	return
}

//export Java_io_v_impl_google_namespace_NamespaceImpl_nativeDelete
func Java_io_v_impl_google_namespace_NamespaceImpl_nativeDelete(jenv *C.JNIEnv, jNamespaceClass C.jclass, goRef C.jlong, jContext C.jobject, jName C.jstring, jDeleteSubtree C.jboolean, jOptions C.jobject, jCallbackObj C.jobject) {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	n := *(*namespace.T)(jutil.GoRefValue(jutil.Ref(goRef)))
	jCallback := jutil.Object(uintptr(unsafe.Pointer(jCallbackObj)))
	context, options, name, deleteSubtree, err := deleteArgs(env, jContext, jOptions, jName, jDeleteSubtree)
	if err != nil {
		jutil.CallbackOnFailure(env, jCallback, err)
		return
	}
	jutil.DoAsyncCall(env, jCallback, func() (jutil.Object, error) {
		return jutil.NullObject, n.Delete(context, name, deleteSubtree, options...)
	})
}

func resolveArgs(env jutil.Env, jName C.jstring, jContext, jOptions C.jobject) (context *context.T, name string, options []naming.NamespaceOpt, err error) {
	context, _, err = jcontext.GoContext(env, jutil.Object(uintptr(unsafe.Pointer(jContext))))
	if err != nil {
		return
	}
	options, err = goNamespaceOptions(env, jutil.Object(uintptr(unsafe.Pointer(jOptions))))
	if err != nil {
		return
	}
	name = jutil.GoString(env, jutil.Object(uintptr(unsafe.Pointer(jName))))
	return
}

func doResolve(n namespace.T, context *context.T, name string, options []naming.NamespaceOpt) (jutil.Object, error) {
	entry, err := n.Resolve(context, name, options...)
	if err != nil {
		return jutil.NullObject, err
	}
	env, freeFunc := jutil.GetEnv()
	defer freeFunc()
	jEntry, err := jutil.JVomCopy(env, entry, JMountEntryClass)
	if err != nil {
		return jutil.NullObject, err
	}
	// Must grab a global reference as we free up the env and all local references that come along
	// with it.
	return jutil.NewGlobalRef(env, jEntry), nil // Un-refed in DoAsyncCall
}

//export Java_io_v_impl_google_namespace_NamespaceImpl_nativeResolve
func Java_io_v_impl_google_namespace_NamespaceImpl_nativeResolve(jenv *C.JNIEnv, jNamespaceClass C.jclass, goRef C.jlong, jContext C.jobject, jName C.jstring, jOptions C.jobject, jCallbackObj C.jobject) {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	n := *(*namespace.T)(jutil.GoRefValue(jutil.Ref(goRef)))
	jCallback := jutil.Object(uintptr(unsafe.Pointer(jCallbackObj)))
	context, name, options, err := resolveArgs(env, jName, jContext, jOptions)
	if err != nil {
		jutil.CallbackOnFailure(env, jCallback, err)
		return
	}
	jutil.DoAsyncCall(env, jCallback, func() (jutil.Object, error) {
		return doResolve(n, context, name, options)
	})
}

func resolveToMountTableArgs(env jutil.Env, jContext, jOptions C.jobject, jName C.jstring) (context *context.T, options []naming.NamespaceOpt, name string, err error) {
	context, _, err = jcontext.GoContext(env, jutil.Object(uintptr(unsafe.Pointer(jContext))))
	if err != nil {
		return
	}
	options, err = goNamespaceOptions(env, jutil.Object(uintptr(unsafe.Pointer(jOptions))))
	if err != nil {
		return
	}
	name = jutil.GoString(env, jutil.Object(uintptr(unsafe.Pointer(jName))))
	return
}

func doResolveToMountTable(n namespace.T, context *context.T, name string, options []naming.NamespaceOpt) (jutil.Object, error) {
	entry, err := n.ResolveToMountTable(context, name, options...)
	if err != nil {
		return jutil.NullObject, err
	}
	env, freeFunc := jutil.GetEnv()
	defer freeFunc()
	jEntry, err := jutil.JVomCopy(env, entry, JMountEntryClass)
	if err != nil {
		return jutil.NullObject, err
	}
	// Must grab a global reference as we free up the env and all local references that come along
	// with it.
	return jutil.NewGlobalRef(env, jEntry), nil // Un-refed in DoAsyncCall
}

//export Java_io_v_impl_google_namespace_NamespaceImpl_nativeResolveToMountTable
func Java_io_v_impl_google_namespace_NamespaceImpl_nativeResolveToMountTable(jenv *C.JNIEnv, jNamespaceClass C.jclass, goRef C.jlong, jContext C.jobject, jName C.jstring, jOptions C.jobject, jCallbackObj C.jobject) {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	n := *(*namespace.T)(jutil.GoRefValue(jutil.Ref(goRef)))
	jCallback := jutil.Object(uintptr(unsafe.Pointer(jCallbackObj)))
	context, options, name, err := resolveToMountTableArgs(env, jContext, jOptions, jName)
	if err != nil {
		jutil.CallbackOnFailure(env, jCallback, err)
		return
	}
	jutil.DoAsyncCall(env, jCallback, func() (jutil.Object, error) {
		return doResolveToMountTable(n, context, name, options)
	})
}

//export Java_io_v_impl_google_namespace_NamespaceImpl_nativeSetCachingPolicy
func Java_io_v_impl_google_namespace_NamespaceImpl_nativeSetCachingPolicy(jenv *C.JNIEnv, jNamespaceClass C.jclass, goRef C.jlong, jDoCaching C.jboolean) C.jboolean {
	n := *(*namespace.T)(jutil.GoRefValue(jutil.Ref(goRef)))
	disable := naming.DisableCache(false)
	if jDoCaching == C.JNI_FALSE {
		disable = naming.DisableCache(true)
	}
	oldCtls := n.CacheCtl(disable)
	var oldDisable naming.DisableCache
	for _, ctl := range oldCtls {
		switch value := ctl.(type) {
		case naming.DisableCache:
			oldDisable = value
		}
	}
	if oldDisable {
		return C.JNI_FALSE
	}
	return C.JNI_TRUE
}

//export Java_io_v_impl_google_namespace_NamespaceImpl_nativeFlushCacheEntry
func Java_io_v_impl_google_namespace_NamespaceImpl_nativeFlushCacheEntry(jenv *C.JNIEnv, jNamespaceClass C.jclass, goRef C.jlong, jContext C.jobject, jName C.jstring) C.jboolean {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	n := *(*namespace.T)(jutil.GoRefValue(jutil.Ref(goRef)))
	context, _, err := jcontext.GoContext(env, jutil.Object(uintptr(unsafe.Pointer(jContext))))
	if err != nil {
		jutil.JThrowV(env, err)
		return C.JNI_FALSE
	}
	result := n.FlushCacheEntry(context, jutil.GoString(env, jutil.Object(uintptr(unsafe.Pointer(jName)))))
	if result {
		return C.JNI_TRUE
	} else {
		return C.JNI_FALSE
	}
}

//export Java_io_v_impl_google_namespace_NamespaceImpl_nativeSetRoots
func Java_io_v_impl_google_namespace_NamespaceImpl_nativeSetRoots(jenv *C.JNIEnv, jNamespaceClass C.jclass, goRef C.jlong, jNames C.jobject) {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	n := *(*namespace.T)(jutil.GoRefValue(jutil.Ref(goRef)))
	names, err := jutil.GoStringList(env, jutil.Object(uintptr(unsafe.Pointer(jNames))))
	if err != nil {
		jutil.JThrowV(env, err)
		return
	}
	err = n.SetRoots(names...)
	if err != nil {
		jutil.JThrowV(env, err)
	}
}

//export Java_io_v_impl_google_namespace_NamespaceImpl_nativeGetRoots
func Java_io_v_impl_google_namespace_NamespaceImpl_nativeGetRoots(jenv *C.JNIEnv, jNamespaceClass C.jclass, goRef C.jlong) C.jobject {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	n := *(*namespace.T)(jutil.GoRefValue(jutil.Ref(goRef)))
	roots := n.Roots()
	jRoots, err := jutil.JStringList(env, roots)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	return C.jobject(unsafe.Pointer(jRoots))
}

func setPermissionsArgs(env jutil.Env, jContext, jPermissions C.jobject, jName, jVersion C.jstring, jOptions C.jobject) (context *context.T, permissions access.Permissions, name, version string, options []naming.NamespaceOpt, err error) {
	context, _, err = jcontext.GoContext(env, jutil.Object(uintptr(unsafe.Pointer(jContext))))
	if err != nil {
		return
	}
	err = jutil.GoVomCopy(env, jutil.Object(uintptr(unsafe.Pointer(jPermissions))), jPermissionsClass, &permissions)
	if err != nil {
		return
	}
	options, err = goNamespaceOptions(env, jutil.Object(uintptr(unsafe.Pointer(jOptions))))
	name = jutil.GoString(env, jutil.Object(uintptr(unsafe.Pointer(jName))))
	version = jutil.GoString(env, jutil.Object(uintptr(unsafe.Pointer(jVersion))))
	return
}

//export Java_io_v_impl_google_namespace_NamespaceImpl_nativeSetPermissions
func Java_io_v_impl_google_namespace_NamespaceImpl_nativeSetPermissions(jenv *C.JNIEnv, jNamespaceClass C.jclass, goRef C.jlong, jContext C.jobject, jName C.jstring, jPermissions C.jobject, jVersion C.jstring, jOptions C.jobject, jCallbackObj C.jobject) {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	n := *(*namespace.T)(jutil.GoRefValue(jutil.Ref(goRef)))
	jCallback := jutil.Object(uintptr(unsafe.Pointer(jCallbackObj)))
	context, permissions, name, version, options, err := setPermissionsArgs(env, jContext, jPermissions, jName, jVersion, jOptions)
	if err != nil {
		jutil.CallbackOnFailure(env, jCallback, err)
		return
	}
	jutil.DoAsyncCall(env, jCallback, func() (jutil.Object, error) {
		return jutil.NullObject, n.SetPermissions(context, name, permissions, version, options...)
	})
}

func getPermissionsArgs(env jutil.Env, jContext C.jobject, jName C.jstring, jOptions C.jobject) (context *context.T, name string, options []naming.NamespaceOpt, err error) {
	context, _, err = jcontext.GoContext(env, jutil.Object(uintptr(unsafe.Pointer(jContext))))
	if err != nil {
		return
	}
	options, err = goNamespaceOptions(env, jutil.Object(uintptr(unsafe.Pointer(jOptions))))
	if err != nil {
		return
	}
	name = jutil.GoString(env, jutil.Object(uintptr(unsafe.Pointer(jName))))
	return
}

func doGetPermissions(n namespace.T, context *context.T, name string, options []naming.NamespaceOpt) (jutil.Object, error) {
	permissions, version, err := n.GetPermissions(context, name, options...)
	if err != nil {
		return jutil.NullObject, err
	}
	env, freeFunc := jutil.GetEnv()
	defer freeFunc()
	jPermissions, err := jutil.JVomCopy(env, permissions, jPermissionsClass)
	if err != nil {
		return jutil.NullObject, err
	}
	result := make(map[jutil.Object]jutil.Object)
	result[jutil.JString(env, version)] = jPermissions
	jResult, err := jutil.JObjectMap(env, result)
	if err != nil {
		return jutil.NullObject, err
	}
	// Must grab a global reference as we free up the env and all local references that come along
	// with it.
	return jutil.NewGlobalRef(env, jResult), nil // Un-refed in DoAsyncCall
}

//export Java_io_v_impl_google_namespace_NamespaceImpl_nativeGetPermissions
func Java_io_v_impl_google_namespace_NamespaceImpl_nativeGetPermissions(jenv *C.JNIEnv, jNamespaceClass C.jclass, goRef C.jlong, jContext C.jobject, jName C.jstring, jOptions C.jobject, jCallbackObj C.jobject) {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	n := *(*namespace.T)(jutil.GoRefValue(jutil.Ref(goRef)))
	jCallback := jutil.Object(uintptr(unsafe.Pointer(jCallbackObj)))
	context, name, options, err := getPermissionsArgs(env, jContext, jName, jOptions)
	if err != nil {
		jutil.CallbackOnFailure(env, jCallback, err)
		return
	}
	jutil.DoAsyncCall(env, jCallback, func() (jutil.Object, error) {
		return doGetPermissions(n, context, name, options)
	})
}

//export Java_io_v_impl_google_namespace_NamespaceImpl_nativeFinalize
func Java_io_v_impl_google_namespace_NamespaceImpl_nativeFinalize(jenv *C.JNIEnv, jNamespace C.jobject, goRef C.jlong) {
	jutil.GoDecRef(jutil.Ref(goRef))
}
