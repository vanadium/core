// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build ignore

package main

import (
	"flag"
	"fmt"
	"os"

	"v.io/x/ref/lib/flags"
)

func main() {
	fl := flags.CreateAndRegister(flag.CommandLine, flags.Runtime, flags.AccessList, flags.Listen)
	flag.PrintDefaults()
	fmt.Printf("Args: %v\n", os.Args)
	if err := fl.Parse(os.Args[1:], nil); err != nil {
		fmt.Println("ERROR: %s", err)
		return
	}
	rtf := fl.RuntimeFlags()
	fmt.Printf("Runtime: Credentials: %s\n", rtf.Credentials)
	fmt.Printf("Runtime: Namespace Roots: %s\n", rtf.NamespaceRoots)
	lf := fl.ListenFlags()
	for _, a := range lf.Addrs {
		fmt.Printf("Listen: Protocol %q, Address %q\n", a.Protocol, a.Address)
	}
	fmt.Printf("Listen: Proxy %q\n", lf.Proxy)
	fmt.Printf("AccessList: %v\n", fl.AccessListFlags())
}
