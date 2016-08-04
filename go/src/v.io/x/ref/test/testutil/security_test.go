// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testutil_test

import (
	"reflect"
	"testing"

	"v.io/x/ref/test/testutil"
)

func TestIDProvider(t *testing.T) {
	idp := testutil.NewIDProvider("foo")
	p := testutil.NewPrincipal()
	if err := idp.Bless(p, "bar"); err != nil {
		t.Fatal(err)
	}
	idpkey, err := idp.PublicKey().MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Roots().Recognized(idpkey, "foo"); err != nil {
		t.Error(err)
	}
	if err := p.Roots().Recognized(idpkey, "foo:bar"); err != nil {
		t.Error(err)
	}
	def, _ := p.BlessingStore().Default()
	peers := p.BlessingStore().ForPeer("anyone_else")
	if def.IsZero() {
		t.Errorf("BlessingStore should have a default blessing")
	}
	if !reflect.DeepEqual(peers, def) {
		t.Errorf("ForPeer(...) returned %v, want %v", peers, def)
	}
	// TODO(ashankar): Implement a security.Call and test the string
	// values as well.
}
