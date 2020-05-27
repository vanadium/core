// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Command fortune is a client to the Fortune interface.
package main

import (
	"flag"
	"log"
	"time"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/x/ref/examples/fortune"
	_ "v.io/x/ref/runtime/factories/generic"
)

var (
	name = flag.String("name", "", "Name of the server to connect to")
	add  = flag.String("add", "", "Fortune string to add")
	has  = flag.String("has", "", "Fortune string to check")
)

func main() {
	ctx, shutdown := v23.Init()
	defer shutdown()

	client := fortune.FortuneClient(*name)

	// Create a child context that will timeout in 60s.
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	switch {
	case *add != "":
		err := client.Add(ctx, *add)
		if err != nil {
			log.Panic("Error adding fortune: ", err)
		}

		log.Println("Fortune added")
	case *has != "":
		exists, err := client.Has(ctx, *has)
		if err != nil {
			log.Panic("Error checking fortune: ", err)
		}

		if exists {
			log.Printf("\"%v\" exists\n", *has)
		} else {
			log.Printf("\"%v\" does not exist\n", *has)
		}
	default:
		// Default for no args is to get a fortune.
		fortune, err := client.Get(ctx)
		if err != nil {
			log.Panic("Error getting fortune: ", err)
		}

		log.Println(fortune)
	}
}
