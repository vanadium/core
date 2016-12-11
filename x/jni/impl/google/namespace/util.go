// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build java android

package namespace

import (
	"v.io/v23/namespace"
	"v.io/v23/naming"
	"v.io/v23/options"
	"v.io/v23/security"

	jutil "v.io/x/jni/util"
)

// #include "jni.h"
import "C"

// JavaNamespace converts the provided Go Namespace into a Java Namespace
// object.
func JavaNamespace(env jutil.Env, namespace namespace.T) (jutil.Object, error) {
	if namespace == nil {
		return jutil.NullObject, nil
	}
	ref := jutil.GoNewRef(&namespace) // Un-refed when the Java NamespaceImpl is finalized.
	jNamespace, err := jutil.NewObject(env, jNamespaceImplClass, []jutil.Sign{jutil.LongSign}, int64(ref))
	if err != nil {
		jutil.GoDecRef(ref)
		return jutil.NullObject, err
	}
	return jNamespace, nil
}

func goNamespaceOptions(env jutil.Env, jOptions jutil.Object) ([]naming.NamespaceOpt, error) {
	var opts []naming.NamespaceOpt
	r, err := jutil.GetBooleanOption(env, jOptions, "io.v.v23.naming.REPLACE_MOUNT")
	if err != nil {
		return nil, err
	}
	opts = append(opts, naming.ReplaceMount(r))
	s, err := jutil.GetBooleanOption(env, jOptions, "io.v.v23.naming.SERVES_MOUNT_TABLE")
	if err != nil {
		return nil, err
	}
	opts = append(opts, naming.ServesMountTable(s))
	l, err := jutil.GetBooleanOption(env, jOptions, "io.v.v23.naming.IS_LEAF")
	if err != nil {
		return nil, err
	}
	opts = append(opts, naming.IsLeaf(l))
	e, err := jutil.GetBooleanOption(env, jOptions, "io.v.v23.SKIP_SERVER_ENDPOINT_AUTHORIZATION")
	if err != nil {
		return nil, err
	}
	if e {
		opts = append(opts, options.NameResolutionAuthorizer{security.AllowEveryone()})
	}
	return opts, nil
}
