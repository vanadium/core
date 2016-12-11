// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build java android

package options

import (
	"time"

	"v.io/v23/naming"
	"v.io/v23/options"
	"v.io/v23/rpc"
	"v.io/v23/security"

	jnamespace "v.io/x/jni/impl/google/namespace"
	jutil "v.io/x/jni/util"
	jsecurity "v.io/x/jni/v23/security"
)

// #include "jni.h"
import "C"

var (
	authorizerSign = jutil.ClassSign("io.v.v23.security.Authorizer")
	mountEntrySign = jutil.ClassSign("io.v.v23.naming.MountEntry")
)

func getAuthorizer(env jutil.Env, obj jutil.Object, field string) (security.Authorizer, error) {
	jAuthorizer, err := jutil.JObjectField(env, obj, field, authorizerSign)
	if err != nil {
		return nil, err
	}

	if !jAuthorizer.IsNull() {
		auth, err := jsecurity.GoAuthorizer(env, jAuthorizer)
		if err != nil {
			return nil, err
		}
		return auth, nil
	}
	return nil, nil
}

func getPreresolved(env jutil.Env, obj jutil.Object) (*naming.MountEntry, error) {
	jMountEntry, err := jutil.JObjectField(env, obj, "preresolved", mountEntrySign)
	if err != nil {
		return nil, err
	}

	if !jMountEntry.IsNull() {
		var mountEntry naming.MountEntry
		if err := jutil.GoVomCopy(env, obj, jnamespace.JMountEntryClass, &mountEntry); err != nil {
			return nil, err
		}
		return &mountEntry, nil
	}
	return nil, nil
}

func getDuration(env jutil.Env, obj jutil.Object, field string) (*time.Duration, error) {
	jDuration, err := jutil.JObjectField(env, obj, field, jutil.DurationSign)
	if err != nil {
		return nil, err
	}

	if !jDuration.IsNull() {
		duration, err := jutil.GoDuration(env, jDuration)
		if err != nil {
			return nil, err
		}
		return &duration, nil
	}
	return nil, nil
}

func GoRpcOpts(env jutil.Env, obj jutil.Object) ([]rpc.CallOpt, error) {
	var opts []rpc.CallOpt

	if opt, err := getAuthorizer(env, obj, "nameResolutionAuthorizer"); err != nil {
		return nil, err
	} else if opt != nil {
		opts = append(opts, options.NameResolutionAuthorizer{opt})
	}

	if opt, err := getAuthorizer(env, obj, "serverAuthorizer"); err != nil {
		return nil, err
	} else if opt != nil {
		opts = append(opts, options.ServerAuthorizer{opt})
	}

	if opt, err := getPreresolved(env, obj); err != nil {
		return nil, err
	} else if opt != nil {
		opts = append(opts, options.Preresolved{opt})
	}

	if opt, err := jutil.JBoolField(env, obj, "noRetry"); err != nil {
		return nil, err
	} else if opt {
		opts = append(opts, options.NoRetry{})
	}

	if opt, err := getDuration(env, obj, "connectionTimeout"); err != nil {
		return nil, err
	} else if opt != nil {
		opts = append(opts, options.ConnectionTimeout(*opt))
	}

	if opt, err := getDuration(env, obj, "channelTimeout"); err != nil {
		return nil, err
	} else if opt != nil {
		opts = append(opts, options.ChannelTimeout(*opt))
	}

	return opts, nil
}

func GoRpcServerOpts(env jutil.Env, obj jutil.Object) ([]rpc.ServerOpt, error) {
	var opts []rpc.ServerOpt

	if opt, err := jutil.JBoolField(env, obj, "servesMountTable"); err != nil {
		return nil, err
	} else {
		opts = append(opts, options.ServesMountTable(opt))
	}

	if opt, err := getDuration(env, obj, "lameDuckTimeout"); err != nil {
		return nil, err
	} else if opt != nil {
		opts = append(opts, options.LameDuckTimeout(*opt))
	}

	if opt, err := jutil.JBoolField(env, obj, "isLeaf"); err != nil {
		return nil, err
	} else {
		opts = append(opts, options.IsLeaf(opt))
	}

	if opt, err := getDuration(env, obj, "channelTimeout"); err != nil {
		return nil, err
	} else if opt != nil {
		opts = append(opts, options.ChannelTimeout(*opt))
	}

	return opts, nil
}
