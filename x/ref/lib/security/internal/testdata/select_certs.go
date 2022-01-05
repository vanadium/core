// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:nobuild
// +build:ignore

package main

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log"
	"os"

	"v.io/x/ref/lib/security/internal"
)

// firefox CA list obtained via brew info ca-certificates, see
// https://wiki.mozilla.org/CA/Included_Certificates.
const FirefoxCAbundle = "/opt/homebrew/Cellar/ca-certificates/2021-10-26/share/ca-certificates/cacert.pem"

func main() {
	blocks, err := internal.LoadCABundleFile(FirefoxCAbundle)
	if err != nil {
		panic(err)
	}

	algos := map[x509.SignatureAlgorithm]int{}
	unique := []*pem.Block{}
	for _, block := range blocks {
		if block.Type != "CERTIFICATE" {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			log.Print(err)
			continue
		}
		algos[cert.SignatureAlgorithm] += 1
		if algos[cert.SignatureAlgorithm] == 1 {
			unique = append(unique, block)
		}
	}

	for algo, n := range algos {
		fmt.Fprintf(os.Stderr, "% 20v: % 3v\n", algo, n)
	}

	for _, u := range unique {
		fmt.Printf("%s\n", pem.EncodeToMemory(u))
	}
}
