// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build java android

package discovery

import (
	"bytes"
	"encoding/binary"

	"v.io/v23/context"
	"v.io/v23/discovery"

	idiscovery "v.io/x/ref/lib/discovery"
	fdiscovery "v.io/x/ref/lib/discovery/factory"
	"v.io/x/ref/lib/discovery/plugins/mock"

	jutil "v.io/x/jni/util"
)

// JavaDiscovery converts a Go discovery instance into a Java discovery instance.
func JavaDiscovery(env jutil.Env, d discovery.T) (jutil.Object, error) {
	ref := jutil.GoNewRef(&d) // Un-refed when jDiscovery is finalized.
	jDiscovery, err := jutil.NewObject(env, jDiscoveryImplClass, []jutil.Sign{jutil.LongSign}, int64(ref))
	if err != nil {
		jutil.GoDecRef(ref)
		return jutil.NullObject, err
	}
	return jDiscovery, nil
}

// javaUpdate converts a Go update instance into a Java update instance.
func javaUpdate(env jutil.Env, update discovery.Update) (jutil.Object, error) {
	jAd, err := jutil.JVomCopy(env, update.Advertisement(), jAdvertisementClass)
	if err != nil {
		return jutil.NullObject, err
	}
	ref := jutil.GoNewRef(&update) // Un-refed when jUpdate is finalized.
	jUpdate, err := jutil.NewObject(env, jUpdateImplClass, []jutil.Sign{jutil.LongSign, jutil.BoolSign, advertisementSign, jutil.LongSign},
		int64(ref), update.IsLost(), jAd, update.Timestamp().UnixNano())
	if err != nil {
		jutil.GoDecRef(ref)
		return jutil.NullObject, err
	}
	return jUpdate, nil
}

// javaUUID converts a Go UUID into a Java UUID instance.
func javaUUID(env jutil.Env, uuid idiscovery.Uuid) (jutil.Object, error) {
	var high, low int64
	buf := bytes.NewReader(uuid)
	binary.Read(buf, binary.BigEndian, &high)
	binary.Read(buf, binary.BigEndian, &low)
	return jutil.NewObject(env, jUUIDClass, []jutil.Sign{jutil.LongSign, jutil.LongSign}, high, low)
}

// injectMockPlugin injects a discovery factory with a mock plugin into a runtime.
func injectMockPlugin(ctx *context.T) error {
	df, err := idiscovery.NewFactory(ctx, mock.New())
	if err != nil {
		return err
	}
	fdiscovery.InjectFactory(df)
	return nil
}
