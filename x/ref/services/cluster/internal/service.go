// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"v.io/v23/context"
	"v.io/v23/rpc"
	"v.io/v23/security"

	"v.io/x/ref/services/cluster"
)

// NewService returns a new clusterAgentService.
func NewService(agent *ClusterAgent) cluster.ClusterAgentAdminServerMethods {
	return &clusterAgentService{agent}
}

// clusterAgentService implements the ClusterAgentAdmin interface.
type clusterAgentService struct {
	agent *ClusterAgent
}

// NewSecret creates a new "secret" that can be used to retrieve extensions of
// the blessings granted on this RPC.
func (i *clusterAgentService) NewSecret(ctx *context.T, call rpc.ServerCall) (string, error) {
	return i.agent.NewSecret(call.GrantedBlessings())
}

// ForgetSecret forgets a secret and its associated blessings.
func (i *clusterAgentService) ForgetSecret(ctx *context.T, _ rpc.ServerCall, secret string) error {
	return i.agent.ForgetSecret(secret)
}

// SeekBlessings retrieves all the blessings associated with a particular
// secret.
func (i *clusterAgentService) SeekBlessings(ctx *context.T, call rpc.ServerCall, secret string) (security.Blessings, error) {
	publicKey := call.Security().RemoteBlessings().PublicKey()
	return i.agent.Bless(secret, publicKey, "x")
}
