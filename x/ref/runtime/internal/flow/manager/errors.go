// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package manager

import "v.io/v23/verror"

var (
	// These were defined in errors.vdl using the ID values
	// below rather than the more conventional Err<Name>.
	errUnknownProtocol           verror.IDAction
	errManagerClosed             verror.IDAction
	errAcceptFailed              verror.IDAction
	errCacheClosed               verror.IDAction
	errConnKilledToFreeResources verror.IDAction
	errInvalidProxyResponse      verror.IDAction
	errListeningWithNullRid      verror.IDAction
	errProxyResponse             verror.IDAction
	errNoBlessingsForPeer        verror.IDAction
	errConnNotInCache            verror.IDAction
)

func init() {
	errUnknownProtocol = verror.NewID("UnknownProtocol")
	errManagerClosed = verror.NewID("ManagerClosed")
	errAcceptFailed = verror.NewID("AcceptFailed")
	errCacheClosed = verror.NewID("CacheClosed")
	errConnKilledToFreeResources = verror.NewID("ConnKilledToFreeResources")
	errInvalidProxyResponse = verror.NewID("InvalidProxyResponse")
	errListeningWithNullRid = verror.NewID("ListeningWithNullRid")
	errProxyResponse = verror.NewID("ProxyResponse")
	errNoBlessingsForPeer = verror.NewID("NoBlessingsForPeer")
	errConnNotInCache = verror.NewID("ConnNotInCache")
}
