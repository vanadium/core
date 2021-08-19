// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"fmt"
	"io"
	"os"
	"strings"

	"v.io/v23/security"
	"v.io/v23/vom"
	seclib "v.io/x/ref/lib/security"
)

// Circuitous route to get to the certificate chains.
// See comments on why security.MarshalBlessings is discouraged.
// Though, a better alternative is worth looking into.
func blessings2wire(b security.Blessings) (security.WireBlessings, error) {
	var wire security.WireBlessings
	data, err := vom.Encode(b)
	if err != nil {
		return wire, err
	}
	err = vom.Decode(data, &wire)
	return wire, err
}

// ReadFileOrStdin will read the data from filename or stdin if
// filename is '-'.
func ReadFileOrStdin(filename string) (string, error) {
	if len(filename) == 0 {
		return "", nil
	}
	var buf []byte
	var err error
	if filename != "-" {
		buf, err = os.ReadFile(filename)
		if err != nil {
			return "", err
		}
	} else {
		buf, err = io.ReadAll(os.Stdin)
	}
	if err != nil {
		return "", err
	}
	return string(buf), nil
}

// WriteFileOrStdout will write the supplied string to filename or to
// stdout if filename is '-'.
func WriteFileOrStdout(filename string, stdout io.Writer, str string) error {
	if filename == "-" {
		fmt.Fprintln(stdout, str)
		return nil
	}
	return os.WriteFile(filename, []byte(str), 0600)
}

// DecodeBlessingsFile will read and decode the blessings in filename or
// from stdin if filename is '-'.
func DecodeBlessingsFile(filename string) (security.Blessings, error) {
	str, err := ReadFileOrStdin(filename)
	if err != nil {
		return security.Blessings{}, err
	}
	return seclib.DecodeBlessingsBase64(str)
}

// EncodeBlessingsFile will encode and write the blessings to filename or
// to stdout if filename is '-'.
func EncodeBlessingsFile(filename string, stdout io.Writer, blessings security.Blessings) error {
	str, err := seclib.EncodeBlessingsBase64(blessings)
	if err != nil {
		return err
	}
	return WriteFileOrStdout(filename, stdout, str)
}

// EncodeBlessingRootsFile will encode and write the blessing roots to
// filename or to stdout if filename is '-'.
func EncodeBlessingRootsFile(filename string, stdout io.Writer, blessings security.Blessings) error {
	out := &strings.Builder{}
	for _, root := range security.RootBlessings(blessings) {
		str, err := seclib.EncodeBlessingsBase64(root)
		if err != nil {
			return err
		}
		out.WriteString(str)
	}
	return WriteFileOrStdout(filename, stdout, out.String())
}
