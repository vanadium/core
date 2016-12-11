// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"fmt"
	"strings"

	"v.io/v23/security"
)

func matchesError(got error, want string) error {
	if (got == nil) && len(want) == 0 {
		return nil
	}
	if got == nil {
		return fmt.Errorf("Got nil error, wanted to match %q", want)
	}
	if !strings.Contains(got.Error(), want) {
		return fmt.Errorf("Got error %q, wanted to match %q", got, want)
	}
	return nil
}

func newPrincipal(selfblessings ...string) (security.Principal, security.Blessings) {
	p, err := NewPrincipal()
	if err != nil {
		panic(err)
	}
	if len(selfblessings) == 0 {
		return p, security.Blessings{}
	}
	var def security.Blessings
	for _, str := range selfblessings {
		b, err := p.BlessSelf(str)
		if err != nil {
			panic(err)
		}
		if def, err = security.UnionOfBlessings(def, b); err != nil {
			panic(err)
		}
	}
	if err := security.AddToRoots(p, def); err != nil {
		panic(err)
	}
	if err := p.BlessingStore().SetDefault(def); err != nil {
		panic(err)
	}
	if _, err := p.BlessingStore().Set(def, security.AllPrincipals); err != nil {
		panic(err)
	}
	return p, def
}

func blessSelf(p security.Principal, name string) security.Blessings {
	b, err := p.BlessSelf(name)
	if err != nil {
		panic(err)
	}
	return b
}

func unionOfBlessings(blessings ...security.Blessings) security.Blessings {
	b, err := security.UnionOfBlessings(blessings...)
	if err != nil {
		panic(err)
	}
	return b
}
