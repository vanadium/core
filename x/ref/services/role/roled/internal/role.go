// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"fmt"
	"strings"
	"time"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/glob"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/verror"

	"v.io/x/ref/services/role"
)

type roleService struct {
	serverConfig *serverConfig
	role         string
	roleConfig   *Config
}

func (i *roleService) SeekBlessings(ctx *context.T, call rpc.ServerCall) (security.Blessings, error) {
	remoteBlessingNames, _ := security.RemoteBlessingNames(ctx, call.Security())
	ctx.Infof("%q.SeekBlessings() called by %q", i.role, remoteBlessingNames)

	members := i.filterNonMembers(remoteBlessingNames)
	if len(members) == 0 {
		// The Authorizer should already have caught that.
		return security.Blessings{}, verror.ErrNoAccess.Errorf(ctx, "access denied")
	}

	extensions := extensions(i.roleConfig, i.role, members)
	caveats, err := caveats(ctx, i.roleConfig)
	if err != nil {
		return security.Blessings{}, err
	}

	return createBlessings(ctx, call.Security(), i.roleConfig, v23.GetPrincipal(ctx), extensions, caveats, i.serverConfig.dischargerLocation)
}

//nolint:golint // API change required.
func (i *roleService) GlobChildren__(ctx *context.T, call rpc.GlobChildrenServerCall, m *glob.Element) error {
	return globChildren(ctx, call, i.serverConfig, m)
}

// filterNonMembers returns only the blessing names that are authorized members
// for the role.
func (i *roleService) filterNonMembers(blessingNames []string) []string {
	var results []string
	for _, name := range blessingNames {
		// It is not enough to know if the pattern is matched by the
		// blessings. We need to know exactly which names matched.
		// These names will be used later to construct the role
		// blessings.
		for _, pattern := range i.roleConfig.Members {
			if pattern.MatchedBy(name) {
				results = append(results, name)
				break
			}
		}
	}
	return results
}

func extensions(config *Config, roleStr string, blessingNames []string) []string {
	// roleStr is the suffix of a veyron object name, but extensions are for
	// blessings, so do the conversion.
	roleStr = strings.ReplaceAll(roleStr, "/", security.ChainSeparator)
	if !config.Extend {
		return []string{roleStr}
	}
	var extensions []string
	for _, b := range blessingNames {
		b = strings.TrimSuffix(b, security.ChainSeparator+role.RoleSuffix)
		extensions = append(extensions, roleStr+security.ChainSeparator+b)
	}
	return extensions
}

func useOrWrapAsInternalErr(ctx *context.T, err error) error {
	if verror.IsAny(err) {
		return err
	}
	return verror.ErrInternal.Errorf(ctx, "internal error: %v", err)
}

func caveats(ctx *context.T, config *Config) ([]security.Caveat, error) {
	var caveats []security.Caveat
	if config.Expiry != "" {
		d, err := time.ParseDuration(config.Expiry)
		if err != nil {
			return nil, useOrCreateErrInternal(ctx, err)
		}
		expiry, err := security.NewExpiryCaveat(time.Now().Add(d))
		if err != nil {
			return nil, useOrCreateErrInternal(ctx, err)
		}
		caveats = append(caveats, expiry)
	}
	if len(config.Peers) != 0 {
		peer, err := security.NewCaveat(security.PeerBlessingsCaveat, config.Peers)
		if err != nil {
			return nil, useOrWrapAsInternalErr(ctx, err)
		}
		caveats = append(caveats, peer)
	}
	return caveats, nil
}

func createBlessings(ctx *context.T, call security.Call, config *Config, principal security.Principal, extensions []string, caveats []security.Caveat, dischargerLocation string) (security.Blessings, error) {
	blessWith := call.LocalBlessings()
	blessWithNames := security.LocalBlessingNames(ctx, call)
	publicKey := call.RemoteBlessings().PublicKey()
	if len(blessWithNames) == 0 {
		return security.Blessings{}, fmt.Errorf("no local blessings")
	}

	var ret security.Blessings
	for _, ext := range extensions {
		cav := caveats
		if config.Audit {
			// TODO(rthellend): This third-party caveat will only work with a single
			// discharger service. We need a way to allow multiple instances of this
			// service to be interchangeable.

			fullNames := make([]string, len(blessWithNames))
			for i, n := range blessWithNames {
				fullNames[i] = n + security.ChainSeparator + ext
			}
			loggingCaveat, err := security.NewCaveat(LoggingCaveat, fullNames)
			if err != nil {
				return security.Blessings{}, useOrCreateErrInternal(ctx, err)
			}
			thirdParty, err := security.NewPublicKeyCaveat(principal.PublicKey(), dischargerLocation, security.ThirdPartyRequirements{
				ReportServer:    true,
				ReportMethod:    true,
				ReportArguments: true,
			}, loggingCaveat)
			if err != nil {
				return security.Blessings{}, useOrWrapAsInternalErr(ctx, err)
			}
			cav = append(cav, thirdParty)
		}
		if len(cav) == 0 {
			// TODO(rthellend,ashankar): the use of unconstrained
			// use is concerning. We should figure out how to get
			// rid of it.
			// Some options:
			//  - have the seeker specify a set of caveats in the
			//    request (and forcefully insert a restrictive one
			//    or fail if the role server thinks that they are
			//    too loose or something).
			//  - have a set of caveats in the config of the role.
			cav = []security.Caveat{security.UnconstrainedUse()}
		}
		b, err := principal.Bless(publicKey, blessWith, ext, cav[0], cav[1:]...)
		if err != nil {
			return security.Blessings{}, useOrCreateErrInternal(ctx, err)
		}
		if ret, err = security.UnionOfBlessings(ret, b); err != nil {
			return ret, useOrWrapAsInternalErr(ctx, err)
		}
	}
	return ret, nil
}
