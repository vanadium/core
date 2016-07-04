// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package compile

import (
	"fmt"
	"strings"
	"testing"
)

func TestXlateErrorFormat(t *testing.T) {
	const pre = "{1:}{2:}"
	tests := []struct {
		Format string
		Want   string
		Err    string
	}{
		{``, pre, ``},
		{`abc`, pre + ` abc`, ``},

		{`{_}{:_}{_:}{:_:}`, pre + ` {_}{:_}{_:}{:_:}`, ``},
		{`{1}{:2}{3:}{:4:}`, pre + ` {1}{:2}{3:}{:4:}`, ``},
		{`{a}{:b}{c:}{:d:}`, pre + ` {3}{:4}{5:}{:6:}`, ``},

		{`A{_}B{:_}C{_:}D{:_:}E`, pre + ` A{_}B{:_}C{_:}D{:_:}E`, ``},
		{`A{1}B{:2}C{3:}D{:4:}E`, pre + ` A{1}B{:2}C{3:}D{:4:}E`, ``},
		{`A{a}B{:b}C{c:}D{:d:}E`, pre + ` A{3}B{:4}C{5:}D{:6:}E`, ``},

		{
			`{_}{1}{a}{:_}{:2}{:b}{_:}{3:}{c:}{:_:}{:4:}{:d:}`,
			pre + ` {_}{1}{3}{:_}{:2}{:4}{_:}{3:}{5:}{:_:}{:4:}{:6:}`,
			``,
		},
		{
			`A{_}B{1}C{a}D{:_}E{:2}F{:b}G{_:}H{3:}I{c:}J{:_:}K{:4:}L{:d:}M`,
			pre + ` A{_}B{1}C{3}D{:_}E{:2}F{:4}G{_:}H{3:}I{5:}J{:_:}K{:4:}L{:6:}M`,
			``,
		},

		{`{ {a}{b}{c} }`, pre + ` { {3}{4}{5} }`, ``},
		{`{x{a}{b}{c}y}`, pre + ` {x{3}{4}{5}y}`, ``},

		{`{foo}`, ``, `unknown param "foo"`},
	}
	params := []*Field{
		{NamePos: NamePos{Name: "a"}},
		{NamePos: NamePos{Name: "b"}},
		{NamePos: NamePos{Name: "c"}},
		{NamePos: NamePos{Name: "d"}},
	}
	for _, test := range tests {
		xlate, err := xlateErrorFormat(test.Format, params)
		if got, want := fmt.Sprint(err), test.Err; !strings.Contains(got, want) {
			t.Errorf(`"%s" got error %q, want substr %q`, test.Format, got, want)
		}
		if got, want := xlate, test.Want; got != want {
			t.Errorf(`"%s" got "%s", want "%s"`, test.Format, got, want)
		}
	}
}
