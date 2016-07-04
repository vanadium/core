// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package wakeuplib implements utilities for wakeup server implementation.
package wakeuplib

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"time"

	"golang.org/x/crypto/nacl/secretbox"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/options"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/security/access"
	"v.io/v23/services/mounttable"
	"v.io/v23/services/wakeup"
	"v.io/v23/verror"
	"v.io/x/ref/services/mounttable/mounttablelib"
)

const (
	pkgPath = "v.io/x/ref/services/wakeup/wakeuplib"
)

var (
	errInvalidName = verror.Register(pkgPath+".errInvalidName", verror.NoRetry, "{1:}{2:} invalid name format {3}; can only be \"mounts/<token>/<svc_name>/{client,server}\" or \"server\" {:_}")
)

// StartServers starts the wakeup server and an associated mounttable,
// returning a function that should be used to stop the servers or an
// error if the servers couldn't be started.
//
// The mounttable is mounted at the provided mountName.  The wakeup
// server is mounted under <mountName>/server.
//
// Provided mtPersistDir directory is used for persisting mounttable
// entries.
//
// Provided sharedKey is used for obscuring wakeup tokens.
//
// Finally, provided wake function is used for waking up the remote
// server using an associated wakeup token and the service name
// component of the mount name.
func StartServers(ctx *context.T, mountName, mtPersistDir string, sharedKey [32]byte, wake func(*context.T, string, string) error) (func(), error) {
	var stopFuncs []func()
	ctx, cancel := context.WithCancel(ctx)
	stop := func() {
		cancel()
		for i := len(stopFuncs) - 1; i >= 0; i-- {
			stopFuncs[i]()
		}
	}
	// Root the mount name.
	if !naming.Rooted(mountName) {
		nsRoots := v23.GetNamespace(ctx).Roots()
		if len(nsRoots) == 0 {
			return nil, fmt.Errorf("Namespace roots not configured")
		}
		mountName = naming.Join(nsRoots[0], mountName)
	}

	// Start the mounttable server.
	mtPermsFile, err := createMTPermsFile(ctx)
	if err != nil {
		return nil, fmt.Errorf("Couldn't create mounttable permissions file: %v", mtPermsFile)
	}
	mtDispatcher, err := mounttablelib.NewMountTableDispatcher(ctx, mtPermsFile, mtPersistDir, "debug")
	if err != nil {
		return nil, err
	}
	mt := &wakeupMT{
		mtDispatcher: mtDispatcher,
		sharedKey:    sharedKey,
		wakeup:       wake,
	}
	ctx, mtServer, err := v23.WithNewDispatchingServer(ctx, mountName, mt, options.ServesMountTable(true))
	if err != nil {
		return nil, err
	}
	stopFuncs = append(stopFuncs, func() {
		<-mtServer.Closed()
	})
	if err := waitServerMounted(mtServer, mountName, 30*time.Second); err != nil {
		stop()
		return nil, err
	}

	// Start the wakeup server.
	w := &wakeupServer{
		wakeupMountPrefix: naming.Join(mountName, "mounts"),
		sharedKey:         [32]byte(sharedKey),
	}
	ctx, wakeupServer, err := v23.WithNewServer(ctx, naming.Join(mountName, "server"), wakeup.WakeUpServer(w), security.AllowEveryone())
	if err != nil {
		stop()
		return nil, err
	}
	stopFuncs = append(stopFuncs, func() {
		<-wakeupServer.Closed()
	})
	if err := waitServerMounted(wakeupServer, naming.Join(mountName, "server"), 30*time.Second); err != nil {
		stop()
		return nil, err
	}
	return stop, nil
}

type wakeupServer struct {
	wakeupMountPrefix string
	sharedKey         [32]byte
}

func (w *wakeupServer) Register(ctx *context.T, call rpc.ServerCall, token string) (string, error) {
	if len(token) == 0 {
		return "", fmt.Errorf("Token cannot be empty.")
	}
	// Convert the token into an obscured name.
	name, err := encodeToken(token, w.sharedKey)
	if err != nil {
		return "", err
	}
	mtName := naming.Join(w.wakeupMountPrefix, name)
	// Create mounttable entry for the name and set permissions on it.
	names, _ := security.RemoteBlessingNames(ctx, call.Security())
	if err := v23.GetNamespace(ctx).SetPermissions(ctx, mtName, mtPerms(names...), ""); err != nil {
		return "", err
	}
	return mtName, nil
}

// Produces: base64(<rand_nonce>secretbox(token))
func encodeToken(token string, sharedKey [32]byte) (string, error) {
	// Generate random nonce.
	var nonce [24]byte
	if _, err := io.ReadFull(rand.Reader, nonce[:]); err != nil {
		return "", err
	}
	s := secretbox.Seal(nil, []byte(token), &nonce, &sharedKey)
	return base64.URLEncoding.EncodeToString(append(nonce[:], s...)), nil
}

func decodeToken(enc string, sharedKey [32]byte) (string, error) {
	data, err := base64.URLEncoding.DecodeString(enc)
	if err != nil {
		return "", fmt.Errorf("Couldn't base64 decode string %q: %v", enc, err)
	}
	var nonce [24]byte
	if n := copy(nonce[:], data); n < 24 {
		return "", fmt.Errorf("Couldn't decode token: encoded length must be at least 24 bytes: %v.", enc)
	}
	b, ok := secretbox.Open(nil, data[24:], &nonce, &sharedKey)
	if !ok {
		return "", fmt.Errorf("Couldn't decrypt token.")
	}
	return string(b), nil
}

type wakeupMT struct {
	wakeupMountPrefix string
	mtDispatcher      rpc.Dispatcher
	sharedKey         [32]byte
	wakeup            func(*context.T, string, string) error
}

// Lookup implements rpc.Dispatcher.Lookup.
// We allow a very restricted set of names to be used, in particular:
//    - "server", used by the clients to talk to the wakeup service,
//    - "mounts/<token_sig>",                     used by the wakeup service to set permissions
//    - "mounts/<token_sig>/<svc_name>/server/*", used by the wakeable service to mount itself
//    - "mounts/<token_sig>/<svc_name>/client/*", used by the by the clients to resolve the
//                                                wakeup-registered services
func (w *wakeupMT) Lookup(ctx *context.T, name string) (interface{}, security.Authorizer, error) {
	if len(name) == 0 || name == "server" {
		// Wakeup service mounting itself or a client lookup for wakeup service.
		return w.mtDispatcher.Lookup(ctx, name)
	}
	if !strings.HasPrefix(name, "mounts/") {
		return nil, nil, verror.New(errInvalidName, ctx, name)
	}
	parts := strings.Split(name, "/")
	if len(parts) < 3 {
		// Wakeup service setting permissions.
		return w.mtDispatcher.Lookup(ctx, name)
	}
	if len(parts) < 4 {
		return nil, nil, verror.New(errInvalidName, ctx, name)
	}
	// Either a wakeable service mounting itself or client resolving a name (with a wakeup).
	if parts[3] != "client" && parts[3] != "server" {
		return nil, nil, verror.New(errInvalidName, ctx, name)
	}
	// Strip away the client/server part of the name: it's used only as a signal to us
	// whether the client or the server is accessing the name (the latter doesn't trigger
	// wakeups).
	isServer := parts[3] == "server"
	parts = append(parts[:3], parts[4:]...)
	mountedName := naming.Join(parts...)
	if isServer {
		return w.mtDispatcher.Lookup(ctx, mountedName)
	}
	// Extract token from the name.
	token, err := decodeToken(parts[1], w.sharedKey)
	if err != nil {
		return nil, nil, err
	}
	serviceName := parts[2]

	// Get the the mounttable server.
	server, authorizer, err := w.mtDispatcher.Lookup(ctx, mountedName)
	if err != nil {
		return nil, nil, err
	}
	mtServer, ok := server.(mounttable.MountTableServerMethods)
	if !ok {
		return nil, nil, fmt.Errorf("invalid mounttable server of type %T", server)
	}
	wakeupMTServer := &wakeupMTContext{
		name:        name,
		token:       token,
		serviceName: serviceName,
		wakeup:      w.wakeup,
		mtServer:    mtServer,
	}
	return mounttable.MountTableServer(wakeupMTServer), authorizer, nil
}

type wakeupMTContext struct {
	name, token, serviceName string
	wakeup                   func(*context.T, string, string) error
	mtServer                 mounttable.MountTableServerMethods
}

func (w *wakeupMTContext) Mount(ctx *context.T, call rpc.ServerCall, server string, ttl uint32, flags naming.MountFlag) error {
	return w.mtServer.Mount(ctx, call, server, ttl, flags)
}

func (w *wakeupMTContext) Unmount(ctx *context.T, call rpc.ServerCall, server string) error {
	return w.mtServer.Unmount(ctx, call, server)
}

func (w *wakeupMTContext) Delete(ctx *context.T, call rpc.ServerCall, deleteSubtree bool) error {
	return w.mtServer.Delete(ctx, call, deleteSubtree)
}

func (w *wakeupMTContext) ResolveStep(ctx *context.T, call rpc.ServerCall) (naming.MountEntry, error) {
	if err := w.wakeup(ctx, w.token, w.serviceName); err != nil {
		ctx.Errorf("couldn't wakeup remote server under name %s: %v", w.name, err)
	}
	return w.mtServer.ResolveStep(ctx, call)
}

func (w *wakeupMTContext) SetPermissions(ctx *context.T, call rpc.ServerCall, perms access.Permissions, version string) error {
	return w.mtServer.SetPermissions(ctx, call, perms, version)
}

func (w *wakeupMTContext) GetPermissions(ctx *context.T, call rpc.ServerCall) (perms access.Permissions, version string, _ error) {
	return w.mtServer.GetPermissions(ctx, call)
}

func mtPerms(blessingNames ...string) access.Permissions {
	p := make(access.Permissions)
	for _, name := range blessingNames {
		p = p.Add(security.BlessingPattern(name), "Admin", "Mount")
	}
	p = p.Add(security.BlessingPattern("..."), "Resolve")
	return p.Normalize()
}

func createMTPermsFile(ctx *context.T) (string, error) {
	data := make(map[string]access.Permissions)
	b, _ := v23.GetPrincipal(ctx).BlessingStore().Default()
	data[""] = mtPerms(security.BlessingNames(v23.GetPrincipal(ctx), b)...)
	mtPermsFile, err := ioutil.TempFile("", "perms")
	if err != nil {
		return "", fmt.Errorf("Error creating permissions file: %v", err)
	}
	if err := json.NewEncoder(mtPermsFile).Encode(data); err != nil {
		return "", err
	}
	if err := mtPermsFile.Sync(); err != nil {
		return "", err
	}
	return mtPermsFile.Name(), nil
}

func waitServerMounted(server rpc.Server, name string, timeout time.Duration) error {
	t := time.After(timeout)
	for true {
		status := server.Status()
		mounted := true
		for _, pub := range status.PublisherStatus {
			if pub.LastState != pub.DesiredState {
				mounted = false
				break
			}
		}
		if mounted {
			return nil
		}
		select {
		case <-status.Dirty:
		case <-t:
			return fmt.Errorf("Couldn't mount at name: %s", name)
		}
	}
	return nil
}
