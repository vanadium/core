// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package security implements utilities for creating and using Vanadium
// security primitives. In particular it provides support for creating and
// using persistent Vanadium principals whose keys and blessings are stored
// in a local file system directory. The blessings may be updated asynchronously
// and reloaded by a running service. Services may run in read-only mode whereby
// they can access updated blessings but cannot generate blessings of their own
// and hence only need access to a public key. Support is also provided
// for hosting private keys in ssh agents rather than the local file system
// and for all signing operations to be carried out by the agent.
package security
