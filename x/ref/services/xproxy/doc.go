// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package xproxy enables services to export (proxy) themselves across networks (behind
// NATs for example).
//
// When clients connect to services through a proxy, the communication between
// the client and service is authenticated and encrypted end-to-end. In other
// words, the proxy will not be privy to the blessings used by the client to
// authenticate with the server nor will it be able to decipher the content of
// the requests and responses between the two. A more detailed explanation
// of this is available at
// https://docs.google.com/document/d/1ONrnxGhOrA8pd0pK0aN5Ued2Q1Eju4zn7vlLt9nzIRA/edit?usp=sharing
//
// For processes to export services through a proxy, the ListenSpec needs
// to include the object name of the proxy service. This can be done via
// the --v23.proxy flag or programmatically with something like:
//
//    ls := rpc.ListenSpec{Proxy: "proxy"}
//    ctx = v23.WithListenSpec(ctx, ls)
//    ctx, server, err := v23.WithNewServer(ctx, "server_name", dispatcher(), authorizer())
//
// Once the server has successfully exported its services through the proxy,
// "server_name" will resolve to an endpoint that directs through the proxy.
// and server.Status().Endpoints will include the endpoint.
package xproxy
