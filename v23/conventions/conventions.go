// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package conventions implements unenforced conventions for Vanadium.
//
// This is still work in progress, the conventions may change. Please do not
// rely on these conventions till this comment is removed!
package conventions

import (
	"strings"

	"v.io/v23/naming"
	"v.io/v23/security"
)

// Blessing represents structured information encoded in a blessing name.
type Blessing struct {
	IdentityProvider string // Name of the identity provider
	User             string // UserID attested to by the identity provider
	Application      string // ApplicationID attested to by the identity provider (may be empty)
	Rest             string // Remaining extensions of the blessing.
}

// String returns the blessing name represented by this structure.
func (b *Blessing) String() string {
	switch b.inferFormat() {
	case formatUser:
		return join(b.IdentityProvider, "u", b.User, b.Rest)
	case formatAppUser:
		return join(b.IdentityProvider, "o", b.Application, b.User, b.Rest)
	default:
		return ""
	}
}

// Home returns the "Home directory" in the global namespace for this Blessing.
func (b *Blessing) Home() string {
	switch b.inferFormat() {
	case formatUser:
		return naming.Join("home", naming.EncodeAsNameElement(join(b.IdentityProvider, "u", b.User)))
	case formatAppUser:
		return naming.Join("home", naming.EncodeAsNameElement(join(b.IdentityProvider, "o", b.Application, b.User)))
	default:
		return ""
	}
}

// UserPattern returns a BlessingPattern that would be matched by a blessing obtainable by the user (irrespective of application).
func (b *Blessing) UserPattern() security.BlessingPattern {
	return pattern(b.IdentityProvider, "u", b.User)
}

// AppUserPattern returns a BlessingPattern that would be matched by a blessing for the user using the same application.
func (b *Blessing) AppUserPattern() security.BlessingPattern {
	// TODO(ashankar): This should change to join(b.IdentityProvider, "a", b.Application, "u", b.User)}
	return pattern(b.IdentityProvider, "o", b.Application, b.User)
}

// AppPattern returns a BlessingPattern that would be matched by all users of the same application.
func (b *Blessing) AppPattern() security.BlessingPattern {
	return pattern(b.IdentityProvider, "o", b.Application)
}

func (b *Blessing) inferFormat() format {
	if len(b.Application) == 0 {
		return formatUser
	}
	return formatAppUser
}

func join(elems ...string) string {
	if len(elems) > 0 && elems[len(elems)-1] == "" {
		elems = elems[:len(elems)-1]
	}
	return strings.Join(elems, security.ChainSeparator)
}
func pattern(elems ...string) security.BlessingPattern {
	for _, e := range elems {
		if len(e) == 0 {
			return security.NoExtension
		}
	}
	return security.BlessingPattern(join(elems...))
}

// format defines the format of the convention used by a blessing name.
type format int

const (
	//nolint:deadcode,unused,varcheck
	formatInfer   format = iota // unknown convention, try to infer it
	formatUser                  // e.g., dev.v.io:u:bugs@bunny.com
	formatAppUser               // e.g., dev.v.io:o:appid:bugs@bunny.com
)

// ParseBlessingNames extracts structured information from the provided blessing names.
// Blessing names that do not adhere to the conventions of this package are ignored.
//
// Typically the set of names to provide would be obtained via a call to
// security.RemoteBlessingNames or security.LocalBlessingNames.
func ParseBlessingNames(blessingNames ...string) []Blessing {
	var ret []Blessing
	for _, n := range blessingNames {
		if b, ok := parseOne(n); ok {
			ret = append(ret, b)
		}
	}
	return ret
}

func parseOne(blessingName string) (Blessing, bool) {
	parts := strings.SplitN(blessingName, security.ChainSeparator, 4)
	if len(parts) < 3 {
		return Blessing{}, false
	}
	var rest string
	if len(parts) > 3 {
		rest = parts[3]
	}
	switch parts[1] {
	case "u":
		return Blessing{
			IdentityProvider: parts[0],
			User:             parts[2],
			Rest:             rest,
		}, true
	case "o":
		if len(rest) == 0 {
			return Blessing{}, false
		}
		// TODO(ashankar): Change this - requiring a 'u' component after the app name?
		moreparts := strings.SplitN(rest, security.ChainSeparator, 2)
		if len(moreparts) > 1 {
			rest = moreparts[1]
		} else {
			rest = ""
		}
		return Blessing{
			IdentityProvider: parts[0],
			User:             moreparts[0],
			Application:      parts[2],
			Rest:             rest,
		}, true
	}
	return Blessing{}, false
}
