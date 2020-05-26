// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package discovery_test

import (
	"testing"

	"v.io/v23/discovery"

	idiscovery "v.io/x/ref/lib/discovery"
	"v.io/x/ref/test"
)

func TestQuery(t *testing.T) {
	ctx, shutdown := test.TestContext()
	defer shutdown()

	ads := []discovery.Advertisement{
		{
			Id:            discovery.AdId{1},
			InterfaceName: "v.io/v23/a",
			Addresses:     []string{"/h1:123/x"},
			Attributes:    map[string]string{"a1": "v1", "a2": "v2"},
		},
		{
			Id:            discovery.AdId{2},
			InterfaceName: "v.io/v23/a",
			Addresses:     []string{"/h2:123/x"},
			Attributes:    map[string]string{"a1": "v2"},
		},
		{
			Id:            discovery.AdId{3},
			InterfaceName: "v.io/v23/b",
			Addresses:     []string{"/h3:123/y"},
			Attributes:    map[string]string{"a1": "v1"},
		},
		{
			Id:            discovery.AdId{4},
			InterfaceName: "v.io/v23/b/c",
			Addresses:     []string{"/h4:123/y"},
		},
	}

	tests := []struct {
		query               string
		targetKey           string
		targetInterfaceName string
		matches             []bool
	}{
		{``, "", "", []bool{true, true, true, true}},
		{`v.InterfaceName="v.io/v23/a"`, "", "v.io/v23/a", []bool{true, true, false, false}},
		{`v.InterfaceName="v.io/v23/c"`, "", "v.io/v23/c", []bool{false, false, false, false}},
		{`v.InterfaceName="v.io/v23/a" AND v.Attributes["a1"]="v1"`, "", "v.io/v23/a", []bool{true, false, false, false}},
		{`v.InterfaceName="v.io/v23/a" AND (v.Attributes["a1"]="v2" OR v.Attributes["a2"] = "v2")`, "", "v.io/v23/a", []bool{true, true, false, false}},
		{`v.InterfaceName="v.io/v23/a" OR v.InterfaceName="v.io/v23/b"`, "", "", []bool{true, true, true, false}},
		{`v.InterfaceName<>"v.io/v23/a"`, "", "", []bool{false, false, true, true}},
		{`v.InterfaceName LIKE "v.io/v23/b%"`, "", "", []bool{false, false, true, true}},
		{`v.InterfaceName="v.io/v23/a" OR v.InterfaceName LIKE "v.io/v23/b%"`, "", "", []bool{true, true, true, true}},
		{`v.Attributes["a1"]="v1"`, "", "", []bool{true, false, true, false}},
		{`k = "04000000000000000000000000000000"`, "04000000000000000000000000000000", "", []bool{false, false, false, true}},
	}

	for i, test := range tests {
		m, err := idiscovery.NewMatcher(ctx, test.query)
		if err != nil {
			t.Errorf("query[%d]: NewMatcher failed: %v", i, err)
			continue
		}

		if m.TargetKey() != test.targetKey {
			t.Errorf("query[%d]: got target key %v; but wanted %v", i, m.TargetKey(), test.targetKey)
		}
		if m.TargetInterfaceName() != test.targetInterfaceName {
			t.Errorf("query[%d]: got target interface name %v; but wanted %v", i, m.TargetInterfaceName(), test.targetInterfaceName)
		}

		for j, ad := range ads {
			tmp := ad
			matched, err := m.Match(&tmp)
			if err != nil {
				t.Errorf("query[%d]: match failed for service[%d]: %v", i, j, err)
				continue
			}
			if matched != test.matches[j] {
				t.Errorf("query[%d]: match returned %t for advertisement[%d]; but wanted %t", i, matched, j, test.matches[j])
			}
		}
	}
}

func TestQueryError(t *testing.T) {
	ctx, shutdown := test.TestContext()
	defer shutdown()

	tests := []string{
		`v.Attachments["k"][0] = 1`, // v.Attachments not allowed.
		`v.Attributes["a1"]=`,
		`v..InterfaceName="v.io/v23/a"`,
		`v.InterfaceName="v.io/v23/a" AND AND v.Attributes["a1"]="v1"`,
	}

	for i, test := range tests {
		if _, err := idiscovery.NewMatcher(ctx, test); err == nil {
			t.Errorf("query[%d]: NewMatcher not failed", i)
		}
	}
}
