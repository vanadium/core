// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xproxy

import (
	"fmt"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/flow"
	"v.io/v23/flow/message"
	"v.io/v23/naming"
	"v.io/v23/security"
	slib "v.io/x/ref/lib/security"
)

type proxyAuthorizer struct{}

func (proxyAuthorizer) AuthorizePeer(
	ctx *context.T,
	localEndpoint, remoteEndpoint naming.Endpoint,
	remoteBlessings security.Blessings,
	remoteDischarges map[string]security.Discharge,
) ([]string, []security.RejectedBlessing, error) {
	return nil, nil, nil
}

func (a proxyAuthorizer) BlessingsForPeer(ctx *context.T, serverBlessings []string) (
	security.Blessings, map[string]security.Discharge, error) {
	blessings := v23.GetPrincipal(ctx).BlessingStore().ForPeer(serverBlessings...)
	discharges, _ := slib.PrepareDischarges(ctx, blessings, serverBlessings, "", nil)
	return blessings, discharges, nil
}

func writeMessage(ctx *context.T, m message.Message, f flow.Flow) error {
	w, err := message.Append(ctx, m, nil)
	if err != nil {
		return err
	}
	_, err = f.WriteMsg(w)
	return err
}

func readMessage(ctx *context.T, f flow.Flow) (message.Message, error) {
	b, err := f.ReadMsg()
	if err != nil {
		return nil, err
	}
	return message.Read(ctx, b)
}

func readProxyResponse(ctx *context.T, f flow.Flow) ([]naming.Endpoint, error) {
	msg, err := readMessage(ctx, f)
	if err != nil {
		return nil, err
	}
	switch m := msg.(type) {
	case *message.ProxyResponse:
		return m.Endpoints, nil
	case *message.ProxyErrorResponse:
		f.Close()
		return nil, ErrorfProxyResponse(ctx, "proxy returned: %v", m.Error)
	default:
		f.Close()
		return nil, ErrorfUnexpectedMessage(ctx, "Unexpected message of type: %s", fmt.Sprintf("%T", m))
	}
}
