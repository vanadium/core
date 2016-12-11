// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vom_test

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"

	"v.io/v23/vdl"
	"v.io/v23/vdl/vdltest"
	"v.io/v23/vom"
)

func TestConvert(t *testing.T) {
	// We run the tests concurrently, sharing a single Type{En,De}coder pair.
	// This tests concurrent usage of the Type{En,De}coder, while still ensuring
	// that the {En,De}coders are used sequentially.
	rType, wType := newPipe()
	encT := vom.NewTypeEncoder(wType)
	decT := vom.NewTypeDecoder(rType)
	decT.Start()
	defer decT.Stop()
	var pending sync.WaitGroup
	// Go race has a limit of 8192 goroutines, so instead of running each test in
	// its own goroutine, we batch up multiple tests into the same goroutine.
	const numGoroutines = 50
	all := vdltest.AllPass()
	numPerGoroutine := len(all) / numGoroutines
	for len(all) > 0 {
		pending.Add(1)
		num := numPerGoroutine
		if len(all) < num {
			num = len(all)
		}
		batch := all[:num]
		all = all[num:]
		go func(batch []vdltest.Entry) {
			defer pending.Done()
			for _, test := range batch {
				// Perform conversion tests with go values.
				name := "[go value] " + test.Name()
				want := rvPtrValue(test.Target).Interface()
				target := reflect.New(test.Target.Type()).Interface()
				source := test.Source.Interface()
				if err := testConvert(target, source, encT, decT); err != nil {
					t.Errorf("%s: %v", name, err)
					continue
				}
				if got, want := target, want; !vdl.DeepEqual(got, want) {
					t.Errorf("%s\nGOT  %#v\nWANT %#v", name, got, want)
					continue
				}
				// Skip conversions from VNamedError into vdl.Value, because verror.E has
				// a weird property that it sets Msg="v.io/v23/verror.Unknown" on its own,
				// which isn't captured in the vdl.Value.
				//
				// TODO(toddw): Fix this weirdness in verror.
				if strings.Contains(test.Name(), "VNamedError") {
					continue
				}
				// Perform conversion tests with vdl.Value.
				name = "[vdl.Value] " + test.Name()
				vvWant, err := vdl.ValueFromReflect(test.Target)
				if err != nil {
					t.Errorf("%s: ValueFromReflect(Target) failed: %v", name, err)
					continue
				}
				vvTarget := vdl.ZeroValue(vvWant.Type())
				vvSource, err := vdl.ValueFromReflect(test.Source)
				if err != nil {
					t.Errorf("%s: ValueFromReflect(Source) failed: %v", name, err)
					continue
				}
				if err := testConvert(vvTarget, vvSource, encT, decT); err != nil {
					t.Errorf("%s: %v", name, err)
					continue
				}
				if got, want := vvTarget, vvWant; !vdl.DeepEqual(got, want) {
					t.Errorf("%s\nGOT  %#v\nWANT %#v", name, got, want)
				}
			}
		}(batch)
	}
	pending.Wait()
}

func testConvert(target, source interface{}, encT *vom.TypeEncoder, decT *vom.TypeDecoder) error {
	if err := testConvertCoder(target, source); err != nil {
		return err
	}
	if err := testConvertSingleShot(target, source); err != nil {
		return err
	}
	return testConvertWithTypeCoder(target, source, encT, decT)
}

func testConvertCoder(target, source interface{}) error {
	var buf bytes.Buffer
	enc := vom.NewEncoder(&buf)
	if err := enc.Encode(source); err != nil {
		return fmt.Errorf("Encode failed: %v", err)
	}
	data := buf.Bytes()
	dec := vom.NewDecoder(&buf)
	if err := dec.Decode(target); err != nil {
		return fmt.Errorf("Decode failed: %v\nDATA %x", err, data)
	}
	return nil
}

func testConvertSingleShot(target, source interface{}) error {
	data, err := vom.Encode(source)
	if err != nil {
		return fmt.Errorf("(single-shot) Encode failed: %v", err)
	}
	if err := vom.Decode(data, target); err != nil {
		return fmt.Errorf("(single-shot) Decode failed: %v\nDATA %x", err, data)
	}
	return nil
}

func testConvertWithTypeCoder(target, source interface{}, encT *vom.TypeEncoder, decT *vom.TypeDecoder) error {
	var buf bytes.Buffer
	enc := vom.NewEncoderWithTypeEncoder(&buf, encT)
	if err := enc.Encode(source); err != nil {
		return fmt.Errorf("(with TypeEncoder) Encode failed: %v", err)
	}
	data := buf.Bytes()
	dec := vom.NewDecoderWithTypeDecoder(&buf, decT)
	if err := dec.Decode(target); err != nil {
		return fmt.Errorf("(with TypeDecoder) Decode failed: %v\nDATA %x", err, data)
	}
	return nil
}
