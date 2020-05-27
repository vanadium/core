// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main_test

// This test assumes the vdl packages under v.io/x/ref/lib/vdl/testdata have been
// compiled using the vdl binary, and runs end-to-end rpc tests against the
// generated output.  It's meant as a final sanity check of the vdl compiler; by
// using the compiled results we're behaving as an end-user would behave.

import (
	"errors"
	"math"
	"reflect"
	"testing"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/rpc"
	"v.io/v23/vdl"
	"v.io/x/ref/lib/vdl/testdata/arith"
	"v.io/x/ref/lib/vdl/testdata/base"
	"v.io/x/ref/test"

	"v.io/v23/vom"
	_ "v.io/x/ref/runtime/factories/generic"
)

var errGenerated = errors.New("generated error")

// serverArith implements the arith.Arith interface.
type serverArith struct{}

func (*serverArith) Add(_ *context.T, _ rpc.ServerCall, a, b int32) (int32, error) {
	return a + b, nil
}

func (*serverArith) DivMod(_ *context.T, _ rpc.ServerCall, a, b int32) (int32, int32, error) {
	return a / b, a % b, nil
}

func (*serverArith) Sub(_ *context.T, _ rpc.ServerCall, args base.Args) (int32, error) {
	return args.A - args.B, nil
}

func (*serverArith) Mul(_ *context.T, _ rpc.ServerCall, nestedArgs base.NestedArgs) (int32, error) {
	return nestedArgs.Args.A * nestedArgs.Args.B, nil
}

func (*serverArith) Count(_ *context.T, call arith.ArithCountServerCall, start int32) error {
	const iterations = 1000
	for i := int32(0); i < iterations; i++ {
		if err := call.SendStream().Send(start + i); err != nil {
			return err
		}
	}
	return nil
}

func (*serverArith) StreamingAdd(_ *context.T, call arith.ArithStreamingAddServerCall) (int32, error) {
	var total int32
	for call.RecvStream().Advance() {
		value := call.RecvStream().Value()
		total += value
		call.SendStream().Send(total) //nolint:errcheck
	}
	return total, call.RecvStream().Err()
}

func (*serverArith) GenError(_ *context.T, _ rpc.ServerCall) error {
	return errGenerated
}

func (*serverArith) QuoteAny(_ *context.T, _ rpc.ServerCall, any *vom.RawBytes) (*vom.RawBytes, error) {
	panic("this should never be called")
}

type serverCalculator struct {
	serverArith
}

func (*serverCalculator) Sine(_ *context.T, _ rpc.ServerCall, angle float64) (float64, error) {
	return math.Sin(angle), nil
}

func (*serverCalculator) Cosine(_ *context.T, _ rpc.ServerCall, angle float64) (float64, error) {
	return math.Cos(angle), nil
}

func (*serverCalculator) Exp(_ *context.T, _ rpc.ServerCall, x float64) (float64, error) {
	return math.Exp(x), nil
}

func (*serverCalculator) On(_ *context.T, _ rpc.ServerCall) error {
	return nil
}

func (*serverCalculator) Off(_ *context.T, _ rpc.ServerCall) error {
	return nil
}

func TestCalculator(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	_, server, err := v23.WithNewServer(ctx, "", arith.CalculatorServer(&serverCalculator{}), nil)
	if err != nil {
		t.Fatal(err)
	}
	root := server.Status().Endpoints[0].Name()
	// Synchronous calls
	calculator := arith.CalculatorClient(root)
	sine, err := calculator.Sine(ctx, 0)
	if err != nil {
		t.Errorf("Sine: got %q but expected no error", err)
	}
	if sine != 0 {
		t.Errorf("Sine: expected 0 got %f", sine)
	}
	cosine, err := calculator.Cosine(ctx, 0)
	if err != nil {
		t.Errorf("Cosine: got %q but expected no error", err)
	}
	if cosine != 1 {
		t.Errorf("Cosine: expected 1 got %f", cosine)
	}

	ar := arith.ArithClient(root)
	sum, err := ar.Add(ctx, 7, 8)
	if err != nil {
		t.Errorf("Add: got %q but expected no error", err)
	}
	if sum != 15 {
		t.Errorf("Add: expected 15 got %d", sum)
	}
	ar = calculator
	sum, err = ar.Add(ctx, 7, 8)
	if err != nil {
		t.Errorf("Add: got %q but expected no error", err)
	}
	if sum != 15 {
		t.Errorf("Add: expected 15 got %d", sum)
	}

	trig := arith.TrigonometryClient(root)
	cosine, err = trig.Cosine(ctx, 0)
	if err != nil {
		t.Errorf("Cosine: got %q but expected no error", err)
	}
	if cosine != 1 {
		t.Errorf("Cosine: expected 1 got %f", cosine)
	}

	// Test auto-generated methods.
	serverStub := arith.CalculatorServer(&serverCalculator{})
	expectDesc(t, serverStub.Describe__(), []rpc.InterfaceDesc{
		{
			Name:    "Calculator",
			PkgPath: "v.io/x/ref/lib/vdl/testdata/arith",
			Embeds: []rpc.EmbedDesc{
				{
					Name:    "Arith",
					PkgPath: "v.io/x/ref/lib/vdl/testdata/arith",
				},
				{
					Name:    "AdvancedMath",
					PkgPath: "v.io/x/ref/lib/vdl/testdata/arith",
				},
			},
			Methods: []rpc.MethodDesc{
				{Name: "On"},
				{Name: "Off", Tags: []*vdl.Value{vdl.StringValue(nil, "offtag")}},
			},
		},
		{
			Name:    "Arith",
			PkgPath: "v.io/x/ref/lib/vdl/testdata/arith",
			Methods: []rpc.MethodDesc{
				{
					Name:    "Add",
					InArgs:  []rpc.ArgDesc{{Name: "a"}, {Name: "b"}},
					OutArgs: []rpc.ArgDesc{{}},
				},
				{
					Name:    "DivMod",
					InArgs:  []rpc.ArgDesc{{Name: "a"}, {Name: "b"}},
					OutArgs: []rpc.ArgDesc{{Name: "quot"}, {Name: "rem"}},
				},
				{
					Name:    "Sub",
					InArgs:  []rpc.ArgDesc{{Name: "args"}},
					OutArgs: []rpc.ArgDesc{{}},
				},
				{
					Name:    "Mul",
					InArgs:  []rpc.ArgDesc{{Name: "nested"}},
					OutArgs: []rpc.ArgDesc{{}},
				},
				{
					Name: "GenError",
					Tags: []*vdl.Value{vdl.StringValue(nil, "foo"), vdl.StringValue(nil, "barz"), vdl.StringValue(nil, "hello"), vdl.IntValue(vdl.Int32Type, 129), vdl.UintValue(vdl.Uint64Type, 0x24)},
				},
				{
					Name:   "Count",
					InArgs: []rpc.ArgDesc{{Name: "start"}},
				},
				{
					Name:    "StreamingAdd",
					OutArgs: []rpc.ArgDesc{{Name: "total"}},
				},
				{
					Name:    "QuoteAny",
					InArgs:  []rpc.ArgDesc{{Name: "a"}},
					OutArgs: []rpc.ArgDesc{{}},
				},
			},
		},
		{
			Name:    "AdvancedMath",
			PkgPath: "v.io/x/ref/lib/vdl/testdata/arith",
			Embeds: []rpc.EmbedDesc{
				{
					Name:    "Trigonometry",
					PkgPath: "v.io/x/ref/lib/vdl/testdata/arith",
				},
				{
					Name:    "Exp",
					PkgPath: "v.io/x/ref/lib/vdl/testdata/arith/exp",
				}},
		},
		{
			Name:    "Trigonometry",
			PkgPath: "v.io/x/ref/lib/vdl/testdata/arith",
			Doc:     "// Trigonometry is an interface that specifies a couple trigonometric functions.",
			Methods: []rpc.MethodDesc{
				{
					Name: "Sine",
					InArgs: []rpc.ArgDesc{
						{Name: "angle", Doc: ``}, // float64
					},
					OutArgs: []rpc.ArgDesc{
						{Name: "", Doc: ``}, // float64
					},
				},
				{
					Name: "Cosine",
					InArgs: []rpc.ArgDesc{
						{Name: "angle", Doc: ``}, // float64
					},
					OutArgs: []rpc.ArgDesc{
						{Name: "", Doc: ``}, // float64
					},
				},
			},
		},
		{
			Name:    "Exp",
			PkgPath: "v.io/x/ref/lib/vdl/testdata/arith/exp",
			Methods: []rpc.MethodDesc{
				{
					Name: "Exp",
					InArgs: []rpc.ArgDesc{
						{Name: "x", Doc: ``}, // float64
					},
					OutArgs: []rpc.ArgDesc{
						{Name: "", Doc: ``}, // float64
					},
				},
			},
		},
	})
}

func TestArith(t *testing.T) { //nolint:gocyclo
	ctx, shutdown := test.V23Init()
	defer shutdown()

	// TODO(bprosnitz) Split this test up -- it is quite long and hard to debug.

	// We try a few types of dispatchers on the server side, to verify that
	// anything dispatching to Arith or an interface embedding Arith (like
	// Calculator) works for a client looking to talk to an Arith service.
	objects := []interface{}{
		arith.ArithServer(&serverArith{}),
		arith.ArithServer(&serverCalculator{}),
		arith.CalculatorServer(&serverCalculator{}),
	}

	for i, obj := range objects {
		_, server, err := v23.WithNewServer(ctx, "", obj, nil)
		if err != nil {
			t.Fatalf("%d: %v", i, err)
		}
		root := server.Status().Endpoints[0].Name()

		// Synchronous calls
		ar := arith.ArithClient(root)
		sum, err := ar.Add(ctx, 7, 8)
		if err != nil {
			t.Errorf("Add: got %q but expected no error", err)
		}
		if sum != 15 {
			t.Errorf("Add: expected 15 got %d", sum)
		}
		q, r, err := ar.DivMod(ctx, 7, 3)
		if err != nil {
			t.Errorf("DivMod: got %q but expected no error", err)
		}
		if q != 2 || r != 1 {
			t.Errorf("DivMod: expected (2,1) got (%d,%d)", q, r)
		}
		diff, err := ar.Sub(ctx, base.Args{A: 7, B: 8})
		if err != nil {
			t.Errorf("Sub: got %q but expected no error", err)
		}
		if diff != -1 {
			t.Errorf("Sub: got %d, expected -1", diff)
		}
		prod, err := ar.Mul(ctx, base.NestedArgs{Args: base.Args{A: 7, B: 8}})
		if err != nil {
			t.Errorf("Mul: got %q, but expected no error", err)
		}
		if prod != 56 {
			t.Errorf("Sub: got %d, expected 56", prod)
		}
		stream, err := ar.Count(ctx, 35)
		if err != nil {
			t.Fatalf("error while executing Count %v", err)
		}

		countIterator := stream.RecvStream()
		for i := int32(0); i < 1000; i++ {
			if !countIterator.Advance() {
				t.Errorf("Error getting value %v", countIterator.Err())
			}
			val := countIterator.Value()
			if val != 35+i {
				t.Errorf("Expected value %d, got %d", 35+i, val)
			}
		}
		if countIterator.Advance() || countIterator.Err() != nil {
			t.Errorf("Reply stream should have been closed %v", countIterator.Err())
		}

		if err := stream.Finish(); err != nil {
			t.Errorf("Count failed with %v", err)
		}

		addStream, err := ar.StreamingAdd(ctx)

		if err != nil {
			t.Errorf("StreamingAdd failed with %v", err)
		}

		go func() {
			sender := addStream.SendStream()
			for i := int32(0); i < 100; i++ {
				if err := sender.Send(i); err != nil {
					t.Errorf("Send error %v", err)
				}
			}
			if err := sender.Close(); err != nil {
				t.Errorf("Close error %v", err)
			}
		}()

		var expectedSum int32
		rStream := addStream.RecvStream()
		for i := int32(0); i < 100; i++ {
			expectedSum += i
			if !rStream.Advance() {
				t.Errorf("Error getting value %v", rStream.Err())
			}
			value := rStream.Value()
			if value != expectedSum {
				t.Errorf("Got %d but expected %d", value, expectedSum)
			}
		}

		if rStream.Advance() || rStream.Err() != nil {
			t.Errorf("Reply stream should have been closed %v", rStream.Err())
		}

		total, err := addStream.Finish()

		if err != nil {
			t.Errorf("Count failed with %v", err)
		}

		if total != expectedSum {
			t.Errorf("Got %d but expexted %d", total, expectedSum)
		}

		if err := ar.GenError(ctx); err == nil {
			t.Errorf("GenError: got %v but expected %v", err, errGenerated)
		}

		// Server-side stubs

		serverStub := arith.ArithServer(&serverArith{})
		expectDesc(t, serverStub.Describe__(), []rpc.InterfaceDesc{
			{
				Name:    "Arith",
				PkgPath: "v.io/x/ref/lib/vdl/testdata/arith",
				Methods: []rpc.MethodDesc{
					{
						Name:    "Add",
						InArgs:  []rpc.ArgDesc{{Name: "a"}, {Name: "b"}},
						OutArgs: []rpc.ArgDesc{{}},
					},
					{
						Name:    "DivMod",
						InArgs:  []rpc.ArgDesc{{Name: "a"}, {Name: "b"}},
						OutArgs: []rpc.ArgDesc{{Name: "quot"}, {Name: "rem"}},
					},
					{
						Name:    "Sub",
						InArgs:  []rpc.ArgDesc{{Name: "args"}},
						OutArgs: []rpc.ArgDesc{{}},
					},
					{
						Name:    "Mul",
						InArgs:  []rpc.ArgDesc{{Name: "nested"}},
						OutArgs: []rpc.ArgDesc{{}},
					},
					{
						Name: "GenError",
						Tags: []*vdl.Value{vdl.StringValue(nil, "foo"), vdl.StringValue(nil, "barz"), vdl.StringValue(nil, "hello"), vdl.IntValue(vdl.Int32Type, 129), vdl.UintValue(vdl.Uint64Type, 0x24)},
					},
					{
						Name:   "Count",
						InArgs: []rpc.ArgDesc{{Name: "start"}},
					},
					{
						Name:    "StreamingAdd",
						OutArgs: []rpc.ArgDesc{{Name: "total"}},
					},
					{
						Name:    "QuoteAny",
						InArgs:  []rpc.ArgDesc{{Name: "a"}},
						OutArgs: []rpc.ArgDesc{{}},
					},
				},
			},
		})
	}
}

func expectDesc(t *testing.T, got, want []rpc.InterfaceDesc) {
	stripDesc(got)
	stripDesc(want)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Describe__ got %#v, want %#v", got, want)
	}
}

func stripDesc(desc []rpc.InterfaceDesc) {
	// Don't bother testing the documentation, to avoid spurious changes.
	for i := range desc {
		desc[i].Doc = ""
		for j := range desc[i].Embeds {
			desc[i].Embeds[j].Doc = ""
		}
		for j := range desc[i].Methods {
			desc[i].Methods[j].Doc = ""
			for k := range desc[i].Methods[j].InArgs {
				desc[i].Methods[j].InArgs[k].Doc = ""
			}
			for k := range desc[i].Methods[j].OutArgs {
				desc[i].Methods[j].OutArgs[k].Doc = ""
			}
			desc[i].Methods[j].InStream.Doc = ""
			desc[i].Methods[j].OutStream.Doc = ""
		}
	}
}
