package internal

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strings"

	"v.io/v23/security"
	"v.io/v23/vom"
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
	return DecodeBlessings(str)
}

// EncodeBlessingsFile will encode and read the blessings to filename or
// to stdout if filename is '-'.
func EncodeBlessingsFile(filename string, stdout io.Writer, blessings security.Blessings) error {
	str, err := EncodeBlessings(blessings)
	if err != nil {
		return err
	}
	return WriteFileOrStdout(filename, stdout, str)
}

func EncodeBlessingRootsFile(filename string, stdout io.Writer, blessings security.Blessings) error {
	out := &strings.Builder{}
	for _, root := range security.RootBlessings(blessings) {
		str, err := EncodeBlessings(root)
		if err != nil {
			return err
		}
		out.WriteString(str)
	}
	return WriteFileOrStdout(filename, stdout, out.String())
}

// EncodeBlessings decodes blessings from the supplied base64url-encoded string.
func DecodeBlessings(encoded string) (security.Blessings, error) {
	var b security.Blessings
	if err := base64urlVomDecode(encoded, &b); err != nil {
		return security.Blessings{}, fmt.Errorf("failed to decode %v: %v", encoded, err)
	}
	return b, nil
}

// EncodeBlessings encodes the supplied blessings as a base64url-encoded string.
func EncodeBlessings(blessings security.Blessings) (string, error) {
	if blessings.IsZero() {
		return "", fmt.Errorf("no blessings found")
	}
	str, err := base64urlVomEncode(blessings)
	if err != nil {
		return "", fmt.Errorf("base64url-vom encoding failed: %v", err)
	}
	return str, nil
}

func base64urlVomEncode(i interface{}) (string, error) {
	buf := &bytes.Buffer{}
	closer := base64.NewEncoder(base64.URLEncoding, buf)
	enc := vom.NewEncoder(closer)
	if err := enc.Encode(i); err != nil {
		return "", err
	}
	// Must close the base64 encoder to flush out any partially written
	// blocks.
	if err := closer.Close(); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func base64urlVomDecode(s string, i interface{}) error {
	b, err := base64.URLEncoding.DecodeString(s)
	if err != nil {
		return err
	}
	dec := vom.NewDecoder(bytes.NewBuffer(b))
	return dec.Decode(i)
}
