// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The following enables go generate to generate the doc.go file.
//go:generate go run v.io/x/lib/cmdline/gendoc .

package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"unicode"

	"v.io/v23/vdl"
	"v.io/v23/vom"
	"v.io/x/lib/cmdline"
)

func main() {
	cmdline.Main(cmdVom)
}

var cmdVom = &cmdline.Command{
	Name:  "vom",
	Short: "helps debug the Vanadium Object Marshaling wire protocol",
	Long: `
Command vom helps debug the Vanadium Object Marshaling wire protocol.
`,
	Children: []*cmdline.Command{cmdDecode, cmdDump},
}

var cmdDecode = &cmdline.Command{
	Runner: cmdline.RunnerFunc(runDecode),
	Name:   "decode",
	Short:  "Decode data encoded in the vom format",
	Long: `
Decode decodes data encoded in the vom format.  If no arguments are provided,
decode reads the data from stdin, otherwise the argument is the data.

By default the data is assumed to be represented in hex, with all whitespace
anywhere in the data ignored.  Use the -data flag to specify other data
representations.

`,
	ArgsName: "[data]",
	ArgsLong: "[data] is the data to decode; if not specified, reads from stdin",
}

var cmdDump = &cmdline.Command{
	Runner: cmdline.RunnerFunc(runDump),
	Name:   "dump",
	Short:  "Dump data encoded in the vom format into formatted output",
	Long: `
Dump dumps data encoded in the vom format, generating formatted output
describing each portion of the encoding.  If no arguments are provided, dump
reads the data from stdin, otherwise the argument is the data.

By default the data is assumed to be represented in hex, with all whitespace
anywhere in the data ignored.  Use the -data flag to specify other data
representations.

Calling "vom dump" with no flags and no arguments combines the default stdin
mode with the default hex mode.  This default mode is special; certain non-hex
characters may be input to represent commands:
  . (period)    Calls Dumper.Status to get the current decoding status.
  ; (semicolon) Calls Dumper.Flush to flush output and start a new message.

This lets you cut-and-paste hex strings into your terminal, and use the commands
to trigger status or flush calls; i.e. a rudimentary debugging UI.

See v.io/v23/vom.Dumper for details on the dump output.
`,
	ArgsName: "[data]",
	ArgsLong: "[data] is the data to dump; if not specified, reads from stdin",
}

var (
	flagDataRep = dataRepHex
)

func init() {
	cmdDecode.Flags.Var(&flagDataRep, "data",
		"Data representation, one of "+fmt.Sprint(dataRepAll))
	cmdDump.Flags.Var(&flagDataRep, "data",
		"Data representation, one of "+fmt.Sprint(dataRepAll))
}

func runDecode(env *cmdline.Env, args []string) error {
	// Convert all inputs into a reader over binary bytes.
	var data string
	switch {
	case len(args) > 1:
		return env.UsageErrorf("too many args")
	case len(args) == 1:
		data = args[0]
	default:
		bytes, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		data = string(bytes)
	}
	binbytes, err := dataToBinaryBytes(data)
	if err != nil {
		return err
	}
	reader := bytes.NewBuffer(binbytes)
	// Decode the binary bytes.
	// TODO(toddw): Add a flag to set a specific type to decode into.
	decoder := vom.NewDecoder(reader)
	var result *vdl.Value
	if err := decoder.Decode(&result); err != nil {
		return err
	}
	fmt.Fprintln(env.Stdout, result)
	if reader.Len() != 0 {
		return fmt.Errorf("%d leftover bytes: % x", reader.Len(), reader.String())
	}
	return nil
}

func runDump(env *cmdline.Env, args []string) error {
	// Handle non-streaming cases.
	switch {
	case len(args) > 1:
		return env.UsageErrorf("too many args")
	case len(args) == 1:
		binbytes, err := dataToBinaryBytes(args[0])
		if err != nil {
			return err
		}
		d, err := vom.Dump(binbytes)
		fmt.Fprintln(env.Stdout, d)
		return err
	}
	// Handle streaming from stdin.
	dumper := vom.NewDumper(vom.NewDumpWriter(env.Stdout))
	defer dumper.Close()
	// Handle simple non-hex cases.
	if flagDataRep == dataRepBinary {
		_, err := io.Copy(dumper, os.Stdin)
		return err
	}
	return runDumpHexStream(dumper)
}

// runDumpHexStream handles the hex stdin-streaming special-case, with commands
// for status and flush.  This is tricky because we need to strip whitespace,
// handle commands where they appear in the stream, and deal with the fact that
// it takes two hex characters to encode a single byte.
//
// The strategy is to run a ReadLoop that reads into a reasonably-sized buffer.
// Inside the ReadLoop we take the buffer, strip whitespace, and keep looping to
// process all data up to each command, and then process the command.  If a
// command appears in the middle of two hex characters representing a byte, we
// send the command first, before sending the byte.
//
// Any leftover non-command single byte is stored in buf and bufStart is set, so
// that the next iteration of ReadLoop can read after those bytes.
func runDumpHexStream(dumper *vom.Dumper) error { //nolint:gocyclo
	buf := make([]byte, 1024)
	bufStart := 0
ReadLoop:
	for {
		n, err := os.Stdin.Read(buf[bufStart:])
		switch {
		case n == 0 && err == io.EOF:
			return nil
		case n == 0 && err != nil:
			return err
		}
		// We may have hex interspersed with spaces and commands.  The strategy is
		// to strip all whitespace, and process each even-sized chunk of hex bytes
		// up to a command or the end of the buffer.
		//
		// Data that appears before a command is written before the command, and
		// data after the command is written after.  But if a command appears in the
		// middle of two hex characters representing a byte, we send the command
		// first, before sending the byte.
		hexbytes := bytes.Map(dropWhitespace, buf[:bufStart+n])
		for len(hexbytes) > 0 {
			end := len(hexbytes)
			cmdIndex := bytes.IndexAny(hexbytes, ".;")
			if cmdIndex != -1 {
				end = cmdIndex
			} else if end == 1 {
				// We have a single non-command byte left in hexbytes; copy it into buf
				// and set bufStart.
				copy(buf, hexbytes[0:1])
				bufStart = 1
				continue ReadLoop
			}
			if end%2 == 1 {
				end-- // Ensure the end is on an even boundary.
			}
			// Write this even-sized chunk of hex bytes to the dumper.
			binbytes, err := hex.DecodeString(string(hexbytes[:end]))
			if err != nil {
				return err
			}
			if _, err := dumper.Write(binbytes); err != nil {
				return err
			}
			// Handle commands.
			if cmdIndex != -1 {
				switch cmd := hexbytes[cmdIndex]; cmd {
				case '.':
					dumper.Status()
				case ';':
					dumper.Flush()
				default:
					return fmt.Errorf("unhandled command %q", cmd)
				}
				// Move data after the command forward.
				copy(hexbytes[cmdIndex:], hexbytes[cmdIndex+1:])
				hexbytes = hexbytes[:len(hexbytes)-1]
			}
			// Move data after the end forward.
			copy(hexbytes, hexbytes[end:])
			hexbytes = hexbytes[:len(hexbytes)-end]
		}
		bufStart = 0
	}
}

func dataToBinaryBytes(data string) ([]byte, error) {
	// Transform all data representations to binary.
	if flagDataRep == dataRepHex {
		// Remove all whitespace out of the hex string.
		binbytes, err := hex.DecodeString(strings.Map(dropWhitespace, data))
		if err != nil {
			return nil, err
		}
		return binbytes, nil
	}
	return []byte(data), nil
}

func dropWhitespace(r rune) rune {
	if unicode.IsSpace(r) {
		return -1
	}
	return r
}
