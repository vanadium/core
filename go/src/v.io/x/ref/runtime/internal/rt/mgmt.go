// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rt

import (
	"encoding/base64"
	"time"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/verror"
	"v.io/v23/vom"

	"v.io/x/ref/lib/exec"
	"v.io/x/ref/lib/mgmt"
	vsecurity "v.io/x/ref/lib/security"
)

const pkgPath = "v.io/x/ref/option/internal/rt"

var (
	errInvalidConfig   = verror.Register(pkgPath+".errInvalidConfig", verror.NoRetry, "{1:}{2:} failed to decode config from the OS environment {:_}")
	errConfigKeyNotSet = verror.Register(pkgPath+".errConfigKeyNotSet", verror.NoRetry, "{1:}{2:} {3} is not set{:_}")
)

func (rt *Runtime) initMgmt(ctx *context.T) error {
	config, err := exec.ReadConfigFromOSEnv()
	if err != nil {
		return verror.New(errInvalidConfig, ctx, err)
	}
	if config == nil {
		// Config was not set.
		return nil
	}

	parentName, err := config.Get(mgmt.ParentNameConfigKey)
	if ctx, err = setListenSpec(ctx, rt, config); err != nil {
		return err
	}
	var blessings security.Blessings
	if b64blessings, err := config.Get(mgmt.AppCycleBlessingsKey); err == nil {
		vombytes, err := base64.URLEncoding.DecodeString(b64blessings)
		if err == nil {
			err = vom.Decode(vombytes, &blessings)
		}
		if err != nil {
			return err
		}
	} else {
		// TODO(caprita,ashankar): This case should only happen when
		// the device manager starts itself up as a child process for
		// testing (see services/device/deviced/internal/impl/device_service.go,
		// specifically testDeviceManager function).
		// FIx that up so it is no longer the case?
		blessings, _ = rt.GetPrincipal(ctx).BlessingStore().Default()
	}
	// Arguably could use the same principal with a different blessing store
	// and blessing roots. However, at the time of this writing there wasn't
	// a particularly convenient way to do so, and this alternative scheme
	// works just as well.
	//
	// Instantiate a principal that will be used by the AppCycleManager
	// which will communicate with (both as a client and a server) only the
	// device manager.
	principal, err := vsecurity.NewPrincipal()
	if err != nil {
		return err
	}
	blessings, err = rt.GetPrincipal(ctx).Bless(principal.PublicKey(), blessings, "instance", security.UnconstrainedUse())
	if err != nil {
		return err
	}
	if err := security.AddToRoots(principal, blessings); err != nil {
		return err
	}
	if err := principal.BlessingStore().SetDefault(blessings); err != nil {
		return err
	}
	if _, err := principal.BlessingStore().Set(blessings, security.AllPrincipals); err != nil {
		return err
	}
	if ctx, err = rt.WithPrincipal(ctx, principal); err != nil {
		return err
	}
	// Fire up the server to receive calls from the parent and also ping the parent.
	ctx, cancel := context.WithCancel(ctx)
	_, server, err := rt.WithNewServer(ctx, "", v23.GetAppCycle(ctx).Remote(), nil)
	if err != nil {
		cancel()
		return err
	}
	if err = rt.callbackToParent(ctx, parentName, server.Status().Endpoints[0].Name()); err != nil {
		cancel()
		<-server.Closed()
		return err
	}
	// cancel() is never called if we return without error.
	return nil
}

func setListenSpec(ctx *context.T, rt *Runtime, config exec.Config) (*context.T, error) {
	protocol, err := config.Get(mgmt.ProtocolConfigKey)
	if err != nil {
		return nil, err
	}
	if protocol == "" {
		return nil, verror.New(errConfigKeyNotSet, ctx, mgmt.ProtocolConfigKey)
	}

	address, err := config.Get(mgmt.AddressConfigKey)
	if err != nil {
		return nil, err
	}
	if address == "" {
		return nil, verror.New(errConfigKeyNotSet, ctx, mgmt.AddressConfigKey)
	}
	return rt.WithListenSpec(ctx, rpc.ListenSpec{Addrs: rpc.ListenAddrs{{protocol, address}}}), nil
}

func (rt *Runtime) callbackToParent(ctx *context.T, parentName, myName string) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	return rt.GetClient(timeoutCtx).Call(timeoutCtx, parentName, "Set", []interface{}{mgmt.AppCycleManagerConfigKey, myName}, nil)
}
