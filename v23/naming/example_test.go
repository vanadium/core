// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package naming_test

import (
	"fmt"

	"v.io/v23/naming"
)

func ExampleEndpoint() {
	// Create an endpoint string for any tcp port on localhost.
	endPoint := naming.FormatEndpoint("tcp", "localhost:0")

	// Create a name for a service, 'example/foo', served on that endpoint.
	name := naming.JoinAddressName(endPoint, "example/foo")
	fmt.Printf("Name: %q\n", name)

	// Create an endpoint string for a global mounttable
	globalMT := naming.FormatEndpoint("tcp", "v.google.com:8080")

	// Create a name for service, published to the mounttable at
	// point 'users/you' serving names with the prefix
	// 'yourservice'
	nameForYou := naming.JoinAddressName(globalMT, naming.Join("users/you", "yourservice"))
	sameNameForYou := naming.JoinAddressName(globalMT, "users/you/yourservice")

	fmt.Printf("Name for you: %q\n", nameForYou)
	fmt.Printf("Same name for you: %q\n", sameNameForYou)

	// Names can be concatenated taking care to handle / correctly.
	fmt.Printf("Simple join: %q\n", naming.Join("a", "b"))
	fmt.Printf("Rooted join: %q\n", naming.Join("/a", "b"))

	// Output:
	// Name: "/@6@tcp@localhost:0@@@@@@/example/foo"
	// Name for you: "/@6@tcp@v.google.com:8080@@@@@@/users/you/yourservice"
	// Same name for you: "/@6@tcp@v.google.com:8080@@@@@@/users/you/yourservice"
	// Simple join: "a/b"
	// Rooted join: "/a/b"
}
