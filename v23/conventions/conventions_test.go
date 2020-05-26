// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conventions

import (
	"reflect"
	"testing"
)

func TestParseBlessingNames(t *testing.T) {
	input := []string{
		"dev.v.io:u:bugs@bunny.com",
		"dev.v.io:u:bugs@bunny.com:frenemy:daffy",
		"dev.v.io:o:836175491023-bhj172jalr081pnjdfpsldjkfh18.apps.googleusercontent.com:mickey@mouse.com",
		"dev.v.io:o:836175491023-bhj172jalr081pnjdfpsldjkfh18.apps.googleusercontent.com:mickey@mouse.com:debugger:friend",
	}
	want := []Blessing{
		{
			IdentityProvider: "dev.v.io",
			User:             "bugs@bunny.com",
		},
		{
			IdentityProvider: "dev.v.io",
			User:             "bugs@bunny.com",
			Rest:             "frenemy:daffy",
		},
		{
			IdentityProvider: "dev.v.io",
			User:             "mickey@mouse.com",
			Application:      "836175491023-bhj172jalr081pnjdfpsldjkfh18.apps.googleusercontent.com",
		},
		{
			IdentityProvider: "dev.v.io",
			User:             "mickey@mouse.com",
			Application:      "836175491023-bhj172jalr081pnjdfpsldjkfh18.apps.googleusercontent.com",
			Rest:             "debugger:friend",
		},
	}
	got := ParseBlessingNames(input...)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Got %#v, want %#v", got, want)
	}
}

func TestUserPattern(t *testing.T) {
	for _, test := range []struct{ Input, Want string }{
		{
			"dev.v.io:u:bruce@wayne.com",
			"dev.v.io:u:bruce@wayne.com",
		},
		{
			"dev.v.io:u:bruce@wayne.com:butler:alfred",
			"dev.v.io:u:bruce@wayne.com",
		},
		{ // App-blessing
			"dev.v.io:o:836175491023-bhj172jalr081pnjdfpsldjkfh18.apps.googleusercontent.com:mickey@mouse.com:debugger:friend",
			"dev.v.io:u:mickey@mouse.com",
		},
	} {
		b := ParseBlessingNames(test.Input)
		if len(b) != 1 {
			t.Errorf("%q: expected a single value, got %d (%v)", test.Input, len(b), b)
			continue
		}
		if got := string(b[0].UserPattern()); got != test.Want {
			t.Errorf("%q: Got %q, want %q", test.Input, got, test.Want)
		}
	}
}

func TestHome(t *testing.T) {
	for _, test := range []struct{ Input, Want string }{
		{
			"dev.v.io:u:bruce@wayne.com",
			"home/dev.v.io:u:bruce@wayne.com",
		},
		{
			"dev.v.io:u:bruce/batman@wayne.com",
			"home/dev.v.io:u:bruce%2Fbatman@wayne.com",
		},
		{
			"dev.v.io:u:bruce@wayne.com:butler:alfred",
			"home/dev.v.io:u:bruce@wayne.com",
		},
		{
			"dev.v.io:o:836175491023-bhj172jalr081pnjdfpsldjkfh18.apps.googleusercontent.com:mickey@mouse.com",
			"home/dev.v.io:o:836175491023-bhj172jalr081pnjdfpsldjkfh18.apps.googleusercontent.com:mickey@mouse.com",
		},
		{
			"dev.v.io:o:836175491023-bhj172jalr081pnjdfpsldjkfh18.apps.googleusercontent.com:mickey@mouse.com:debugger:friend",
			"home/dev.v.io:o:836175491023-bhj172jalr081pnjdfpsldjkfh18.apps.googleusercontent.com:mickey@mouse.com",
		},
	} {
		b := ParseBlessingNames(test.Input)
		if len(b) != 1 {
			t.Errorf("%q: expected a single value, got %d (%v)", test.Input, len(b), b)
			continue
		}
		if got := b[0].Home(); got != test.Want {
			t.Errorf("%q: Got %q, want %q", test.Input, got, test.Want)
		}
	}
}

func TestAppPattern(t *testing.T) {
	for _, test := range []struct{ Input, Want string }{
		{ // User-centric blessing, cannot restrict to an app
			"dev.v.io:u:bruce@wayne.com",
			"$",
		},
		{
			"dev.v.io:o:836175491023-bhj172jalr081pnjdfpsldjkfh18.apps.googleusercontent.com:mickey@mouse.com:debugger:friend",
			"dev.v.io:o:836175491023-bhj172jalr081pnjdfpsldjkfh18.apps.googleusercontent.com",
		},
	} {
		b := ParseBlessingNames(test.Input)
		if len(b) != 1 {
			t.Errorf("%q: expected a single value, got %d (%v)", test.Input, len(b), b)
			continue
		}
		if got := string(b[0].AppPattern()); got != test.Want {
			t.Errorf("%q: Got %q, want %q", test.Input, got, test.Want)
		}
	}
}

func TestAppUserPattern(t *testing.T) {
	for _, test := range []struct{ Input, Want string }{
		{ // User-centric blessing, cannot restrict to an app
			"dev.v.io:u:bruce@wayne.com",
			"$",
		},
		{
			"dev.v.io:o:836175491023-bhj172jalr081pnjdfpsldjkfh18.apps.googleusercontent.com:mickey@mouse.com:debugger:friend",
			"dev.v.io:o:836175491023-bhj172jalr081pnjdfpsldjkfh18.apps.googleusercontent.com:mickey@mouse.com",
		},
	} {
		b := ParseBlessingNames(test.Input)
		if len(b) != 1 {
			t.Errorf("%q: expected a single value, got %d (%v)", test.Input, len(b), b)
			continue
		}
		if got := string(b[0].AppUserPattern()); got != test.Want {
			t.Errorf("%q: Got %q, want %q", test.Input, got, test.Want)
		}
	}
}

func TestUnconventionalParseBlessingName(t *testing.T) {
	for _, test := range []string{
		"foo",
		"foo:unrecognized:user",
		"dev.v.io:u",
		"dev.v.io:o",
		"dev.v.io:o:app",
		// App-blessing without any extension for the user. Arguably, this should be okay?
		"dev.v.io:o:836175491023-bhj172jalr081pnjdfpsldjkfh18.apps.googleusercontent.com",
	} {
		if b := ParseBlessingNames(test); len(b) != 0 {
			t.Errorf("ParseBlessingNames succeeded for unconventional %q: %v", test, b)
		}
	}
}
