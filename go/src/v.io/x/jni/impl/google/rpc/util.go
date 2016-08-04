// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build java android

package rpc

import (
	"fmt"
	"net"
	"runtime"

	"v.io/v23/rpc"

	jutil "v.io/x/jni/util"
)

// #include "jni.h"
import "C"

// JavaServer converts the provided Go Server into a Java Server object.
func JavaServer(env jutil.Env, server rpc.Server) (jutil.Object, error) {
	if server == nil {
		return jutil.NullObject, fmt.Errorf("Go Server value cannot be nil")
	}
	ref := jutil.GoNewRef(&server) // Un-refed when the Java Server object is finalized.
	jServer, err := jutil.NewObject(env, jServerImplClass, []jutil.Sign{jutil.LongSign}, int64(ref))
	if err != nil {
		jutil.GoDecRef(ref)
		return jutil.NullObject, err
	}
	return jServer, nil
}

// JavaClient converts the provided Go client into a Java Client object.
func JavaClient(env jutil.Env, client rpc.Client) (jutil.Object, error) {
	if client == nil {
		return jutil.NullObject, nil
	}
	ref := jutil.GoNewRef(&client) // Un-refed when the Java Client object is finalized.
	jClient, err := jutil.NewObject(env, jClientImplClass, []jutil.Sign{jutil.LongSign}, int64(ref))
	if err != nil {
		jutil.GoDecRef(ref)
		return jutil.NullObject, err
	}
	return jClient, nil
}

// javaStreamServerCall converts the provided Go serverCall into a Java StreamServerCall
// object.
func javaStreamServerCall(env jutil.Env, jContext jutil.Object, call rpc.StreamServerCall) (jutil.Object, error) {
	if call == nil {
		return jutil.NullObject, fmt.Errorf("Go StreamServerCall value cannot be nil")
	}
	jStream, err := javaStream(env, jContext, call)
	if err != nil {
		return jutil.NullObject, err
	}
	jServerCall, err := JavaServerCall(env, call)
	if err != nil {
		return jutil.NullObject, err
	}
	serverCallSign := jutil.ClassSign("io.v.v23.rpc.ServerCall")
	ref := jutil.GoNewRef(&call) // Un-refed when the Java StreamServerCall object is finalized.
	jStreamServerCall, err := jutil.NewObject(env, jStreamServerCallImplClass, []jutil.Sign{jutil.LongSign, streamSign, serverCallSign}, int64(ref), jStream, jServerCall)
	if err != nil {
		jutil.GoDecRef(ref)
		return jutil.NullObject, err
	}
	return jStreamServerCall, nil
}

// javaCall converts the provided Go Call value into a Java Call object.
func javaCall(env jutil.Env, jContext jutil.Object, call rpc.ClientCall) (jutil.Object, error) {
	if call == nil {
		return jutil.NullObject, fmt.Errorf("Go Call value cannot be nil")
	}
	jStream, err := javaStream(env, jContext, call)
	if err != nil {
		return jutil.NullObject, err
	}
	ref := jutil.GoNewRef(&call) // Un-refed when the Java Call object is finalized.
	jCall, err := jutil.NewObject(env, jClientCallImplClass, []jutil.Sign{contextSign, jutil.LongSign, streamSign}, jContext, int64(ref), jStream)
	if err != nil {
		jutil.GoDecRef(ref)
		return jutil.NullObject, err
	}
	return jCall, nil
}

// javaStream converts the provided Go stream into a Java Stream object.
func javaStream(env jutil.Env, jContext jutil.Object, stream rpc.Stream) (jutil.Object, error) {
	ref := jutil.GoNewRef(&stream) // Un-refed when the Java stream object is finalized.
	jStream, err := jutil.NewObject(env, jStreamImplClass, []jutil.Sign{contextSign, jutil.LongSign}, jContext, int64(ref))
	if err != nil {
		jutil.GoDecRef(ref)
		return jutil.NullObject, err
	}
	return jStream, nil
}

// JavaServerStatus converts the provided rpc.ServerStatus value into a Java
// ServerStatus object.
func JavaServerStatus(env jutil.Env, status rpc.ServerStatus) (jutil.Object, error) {
	// Create Java state enum value.
	jState, err := JavaServerState(env, status.State)
	if err != nil {
		return jutil.NullObject, err
	}

	// Create Java array of publisher entries.
	pubarr := make([]jutil.Object, len(status.PublisherStatus))
	for i, e := range status.PublisherStatus {
		var err error
		if pubarr[i], err = JavaPublisherEntry(env, e); err != nil {
			return jutil.NullObject, err
		}
	}
	jPublisherStatus, err := jutil.JObjectArray(env, pubarr, jPublisherEntryClass)
	if err != nil {
		return jutil.NullObject, err
	}

	// Create an array of endpoint strings.
	eps := make([]string, len(status.Endpoints))
	for i, ep := range status.Endpoints {
		eps[i] = ep.String()
	}

	lnErrors := make(map[jutil.Object]jutil.Object)
	for addr, lerr := range status.ListenErrors {
		jAddr, err := JavaListenAddr(env, addr.Protocol, addr.Address)
		if err != nil {
			return jutil.NullObject, err
		}
		jVExp, err := jutil.JVException(env, lerr)
		if err != nil {
			return jutil.NullObject, err
		}
		lnErrors[jAddr] = jVExp
	}
	jLnErrors, err := jutil.JObjectMap(env, lnErrors)
	if err != nil {
		return jutil.NullObject, err
	}

	proxyErrors := make(map[jutil.Object]jutil.Object)
	for s, perr := range status.ProxyErrors {
		jVExp, err := jutil.JVException(env, perr)
		if err != nil {
			return jutil.NullObject, err
		}
		proxyErrors[jutil.JString(env, s)] = jVExp
	}
	jProxyErrors, err := jutil.JObjectMap(env, proxyErrors)
	if err != nil {
		return jutil.NullObject, err
	}

	// Create final server status.
	publisherEntrySign := jutil.ClassSign("io.v.v23.rpc.PublisherEntry")
	jServerStatus, err := jutil.NewObject(env, jServerStatusClass, []jutil.Sign{serverStateSign, jutil.BoolSign, jutil.ArraySign(publisherEntrySign), jutil.ArraySign(jutil.StringSign), jutil.MapSign, jutil.MapSign}, jState, status.ServesMountTable, jPublisherStatus, eps, jLnErrors, jProxyErrors)
	if err != nil {
		return jutil.NullObject, err
	}
	return jServerStatus, nil
}

// JavaServerState converts the provided rpc.ServerState value into a Java
// ServerState enum.
func JavaServerState(env jutil.Env, state rpc.ServerState) (jutil.Object, error) {
	var name string
	switch state {
	case rpc.ServerActive:
		name = "SERVER_ACTIVE"
	case rpc.ServerStopping:
		name = "SERVER_STOPPING"
	case rpc.ServerStopped:
		name = "SERVER_STOPPED"
	default:
		return jutil.NullObject, fmt.Errorf("Unrecognized state: %d", state)
	}
	return jutil.CallStaticObjectMethod(env, jServerStateClass, "valueOf", []jutil.Sign{jutil.StringSign}, serverStateSign, name)
}

// JavaPublisherEntry converts the provided rpc.PublisherEntry value into a Java
// PublisherEntry object.
// TODO(suharshs): Add PublisherState to the java publisher entry? May not make sense, since there is no
// notification channel in Java.
func JavaPublisherEntry(env jutil.Env, entry rpc.PublisherEntry) (jutil.Object, error) {
	jStatus, err := jutil.NewObject(env, jPublisherEntryClass, []jutil.Sign{jutil.StringSign, jutil.StringSign, jutil.DateTimeSign, jutil.VExceptionSign, jutil.DurationSign, jutil.DateTimeSign, jutil.VExceptionSign}, entry.Name, entry.Server, entry.LastMount, entry.LastMountErr, entry.TTL, entry.LastUnmount, entry.LastUnmountErr)
	if err != nil {
		return jutil.NullObject, err
	}
	return jStatus, nil
}

// GoListenSpec converts the provided Java ListenSpec into a Go ListenSpec.
func GoListenSpec(env jutil.Env, jSpec jutil.Object) (rpc.ListenSpec, error) {
	jAddrs, err := jutil.CallObjectMethod(env, jSpec, "getAddresses", nil, jutil.ArraySign(listenAddrSign))
	if err != nil {
		return rpc.ListenSpec{}, err
	}
	addrs, err := GoListenAddrs(env, jAddrs)
	if err != nil {
		return rpc.ListenSpec{}, err
	}
	proxy, err := jutil.CallStringMethod(env, jSpec, "getProxy", nil)
	if err != nil {
		return rpc.ListenSpec{}, err
	}
	jChooser, err := jutil.CallObjectMethod(env, jSpec, "getChooser", nil, addressChooserSign)
	if err != nil {
		return rpc.ListenSpec{}, err
	}
	chooser := GoAddressChooser(env, jChooser)
	return rpc.ListenSpec{
		Addrs:          addrs,
		Proxy:          proxy,
		AddressChooser: chooser,
	}, nil
}

// GoListenSpec converts the provided Go ListenSpec into a Java ListenSpec.
func JavaListenSpec(env jutil.Env, spec rpc.ListenSpec) (jutil.Object, error) {
	jAddrs, err := JavaListenAddrArray(env, spec.Addrs)
	if err != nil {
		return jutil.NullObject, err
	}
	jChooser, err := JavaAddressChooser(env, spec.AddressChooser)
	if err != nil {
		return jutil.NullObject, err
	}
	addressSign := jutil.ClassSign("io.v.v23.rpc.ListenSpec$Address")
	jSpec, err := jutil.NewObject(env, jListenSpecClass, []jutil.Sign{jutil.ArraySign(addressSign), jutil.StringSign, addressChooserSign}, jAddrs, spec.Proxy, jChooser)
	if err != nil {
		return jutil.NullObject, err
	}
	return jSpec, nil
}

func JavaListenAddr(env jutil.Env, protocol, address string) (jutil.Object, error) {
	return jutil.NewObject(env, jListenSpecAddressClass, []jutil.Sign{jutil.StringSign, jutil.StringSign}, protocol, address)
}

// JavaListenAddrArray converts Go rpc.ListenAddrs into a Java
// ListenSpec$Address array.
func JavaListenAddrArray(env jutil.Env, addrs rpc.ListenAddrs) (jutil.Object, error) {
	addrarr := make([]jutil.Object, len(addrs))
	for i, addr := range addrs {
		var err error
		if addrarr[i], err = JavaListenAddr(env, addr.Protocol, addr.Address); err != nil {
			return jutil.NullObject, err
		}
	}
	return jutil.JObjectArray(env, addrarr, jListenSpecAddressClass)
}

// GoListenAddrs converts Java ListenSpec$Address array into a Go
// rpc.ListenAddrs value.
func GoListenAddrs(env jutil.Env, jAddrs jutil.Object) (rpc.ListenAddrs, error) {
	addrarr, err := jutil.GoObjectArray(env, jAddrs)
	if err != nil {
		return nil, err
	}
	addrs := make(rpc.ListenAddrs, len(addrarr))
	for i, jAddr := range addrarr {
		var err error
		addrs[i].Protocol, err = jutil.CallStringMethod(env, jAddr, "getProtocol", nil)
		if err != nil {
			return nil, err
		}
		addrs[i].Address, err = jutil.CallStringMethod(env, jAddr, "getAddress", nil)
		if err != nil {
			return nil, err
		}
	}
	return addrs, nil
}

// JavaServerCall converts a Go rpc.ServerCall into a Java ServerCall object.
func JavaServerCall(env jutil.Env, serverCall rpc.ServerCall) (jutil.Object, error) {
	ref := jutil.GoNewRef(&serverCall) // Un-refed when the Java ServerCall object is finalized.
	jServerCall, err := jutil.NewObject(env, jServerCallImplClass, []jutil.Sign{jutil.LongSign}, int64(ref))
	if err != nil {
		jutil.GoDecRef(ref)
		return jutil.NullObject, err
	}
	return jServerCall, nil
}

// JavaNetworkAddress converts a Go net.Addr into a Java NetworkAddress object.
func JavaNetworkAddress(env jutil.Env, addr net.Addr) (jutil.Object, error) {
	return jutil.NewObject(env, jNetworkAddressClass, []jutil.Sign{jutil.StringSign, jutil.StringSign}, addr.Network(), addr.String())
}

type jniAddr struct {
	network, addr string
}

func (a *jniAddr) Network() string {
	return a.network
}

func (a *jniAddr) String() string {
	return a.addr
}

// GoNetworkAddress converts a Java NetworkAddress object into Go net.Addr.
func GoNetworkAddress(env jutil.Env, jAddr jutil.Object) (net.Addr, error) {
	network, err := jutil.CallStringMethod(env, jAddr, "network", nil)
	if err != nil {
		return nil, err
	}
	addr, err := jutil.CallStringMethod(env, jAddr, "address", nil)
	if err != nil {
		return nil, err
	}
	return &jniAddr{network, addr}, nil
}

// JavaNetworkAddressArray converts a Go slice of net.Addr values into a Java
// array of NetworkAddress objects.
func JavaNetworkAddressArray(env jutil.Env, addrs []net.Addr) (jutil.Object, error) {
	arr := make([]jutil.Object, len(addrs))
	for i, addr := range addrs {
		var err error
		if arr[i], err = JavaNetworkAddress(env, addr); err != nil {
			return jutil.NullObject, err
		}
	}
	return jutil.JObjectArray(env, arr, jNetworkAddressClass)
}

// GoNetworkAddressArray converts a Java array of NetworkAddress objects into a
// Go slice of net.Addr values.
func GoNetworkAddressArray(env jutil.Env, jAddrs jutil.Object) ([]net.Addr, error) {
	arr, err := jutil.GoObjectArray(env, jAddrs)
	if err != nil {
		return nil, err
	}
	ret := make([]net.Addr, len(arr))
	for i, jAddr := range arr {
		var err error
		if ret[i], err = GoNetworkAddress(env, jAddr); err != nil {
			return nil, err
		}
	}
	return ret, nil
}

// JavaAddressChooser converts a Go address chooser function into a Java
// AddressChooser object.
func JavaAddressChooser(env jutil.Env, chooser rpc.AddressChooser) (jutil.Object, error) {
	ref := jutil.GoNewRef(&chooser) // Un-refed when the Java AddressChooser object is finalized.
	jAddressChooser, err := jutil.NewObject(env, jAddressChooserImplClass, []jutil.Sign{jutil.LongSign}, int64(ref))
	if err != nil {
		jutil.GoDecRef(ref)
		return jutil.NullObject, err
	}
	return jAddressChooser, nil
}

type jniAddressChooser struct {
	jChooser jutil.Object
}

func (chooser *jniAddressChooser) ChooseAddresses(protocol string, candidates []net.Addr) ([]net.Addr, error) {
	env, freeFunc := jutil.GetEnv()
	defer freeFunc()
	jCandidates, err := JavaNetworkAddressArray(env, candidates)
	if err != nil {
		return nil, err
	}
	addrsSign := jutil.ArraySign(jutil.ClassSign("io.v.v23.rpc.NetworkAddress"))
	jAddrs, err := jutil.CallObjectMethod(env, chooser.jChooser, "choose", []jutil.Sign{jutil.StringSign, addrsSign}, addrsSign, protocol, jCandidates)
	if err != nil {
		return nil, err
	}
	return GoNetworkAddressArray(env, jAddrs)
}

// GoAddressChooser converts a Java AddressChooser object into a Go address
// chooser function.
func GoAddressChooser(env jutil.Env, jChooser jutil.Object) rpc.AddressChooser {
	// Reference Java chooser; it will be de-referenced when the go function
	// created below is garbage-collected (through the finalizer callback we
	// setup just below).
	chooser := &jniAddressChooser{
		jChooser: jutil.NewGlobalRef(env, jChooser),
	}
	runtime.SetFinalizer(chooser, func(chooser *jniAddressChooser) {
		env, freeFunc := jutil.GetEnv()
		defer freeFunc()
		jutil.DeleteGlobalRef(env, chooser.jChooser)
	})
	return chooser
}
