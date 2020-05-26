// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package opconst

import (
	"fmt"
	"math/big"
	"testing"

	"v.io/v23/vdl"
)

const (
	noType         = "must be assigned a type"
	cantConvert    = "can't convert"
	overflows      = "overflows"
	underflows     = "underflows"
	losesPrecision = "loses precision"
	//nolint:deadcode,structcheck,varcheck
	nonzeroImaginary = "nonzero imaginary"
	notSupported     = "not supported"
	divByZero        = "divide by zero"
)

var (
	bi0           = new(big.Int) //nolint:deadcode,structcheck,unused,varcheck
	bi1, bi2, bi3 = big.NewInt(1), big.NewInt(2), big.NewInt(3)
	bi4, bi5, bi6 = big.NewInt(4), big.NewInt(5), big.NewInt(6)
	bi7           = big.NewInt(7)
	//nolint:deadcode,structcheck,unused,varcheck
	bi8, bi9       = big.NewInt(8), big.NewInt(9)
	biNeg1, biNeg2 = big.NewInt(-1), big.NewInt(-2)

	//nolint:deadcode,structcheck,unused,varcheck
	br0           = new(big.Rat)
	br1, br2, br3 = big.NewRat(1, 1), big.NewRat(2, 1), big.NewRat(3, 1)
	br4, br5, br6 = big.NewRat(4, 1), big.NewRat(5, 1), big.NewRat(6, 1)
	br7           = big.NewRat(7, 1)
	//nolint:deadcode,structcheck,unused,varcheck
	br8, br9 = big.NewRat(8, 1), big.NewRat(9, 1)
	brNeg1   = big.NewRat(-1, 1)
	//nolint:deadcode,structcheck,unused,varcheck
	brNeg2 = big.NewRat(-2, 1)
)

func boolConst(t *vdl.Type, x bool) Const     { return FromValue(vdl.BoolValue(t, x)) }
func stringConst(t *vdl.Type, x string) Const { return FromValue(vdl.StringValue(t, x)) }
func bytesConst(t *vdl.Type, x string) Const  { return FromValue(vdl.BytesValue(t, []byte(x))) }
func intConst(t *vdl.Type, x int64) Const     { return FromValue(vdl.IntValue(t, x)) }
func uintConst(t *vdl.Type, x uint64) Const   { return FromValue(vdl.UintValue(t, x)) }
func floatConst(t *vdl.Type, x float64) Const { return FromValue(vdl.FloatValue(t, x)) }
func structNumConst(t *vdl.Type, x float64) Const {
	return FromValue(structNumValue(t, sn{"A", x}))
}

func constEqual(a, b Const) bool {
	if !a.IsValid() && !b.IsValid() {
		return true
	}
	res, err := EvalBinary(EQ, a, b)
	if err != nil || !res.IsValid() {
		return false
	}
	val, err := res.ToValue()
	return err == nil && val != nil && val.Kind() == vdl.Bool && val.Bool()
}

func TestConstInvalid(t *testing.T) {
	x := Const{}
	if x.IsValid() {
		t.Errorf("zero Const IsValid")
	}
	if got, want := x.String(), "invalid"; got != want {
		t.Errorf("ToValue got string %v, want %v", got, want)
	}
	{
		value, err := x.ToValue()
		if value != nil {
			t.Errorf("ToValue got valid value %v, want nil", value)
		}
		if got, want := fmt.Sprint(err), "invalid const"; got != want {
			t.Errorf("ToValue got error %q, want %q", got, want)
		}
	}
	{
		result, err := x.Convert(vdl.BoolType)
		if result.IsValid() {
			t.Errorf("Convert got valid result %v, want invalid", result)
		}
		if got, want := fmt.Sprint(err), "invalid const"; got != want {
			t.Errorf("Convert got error %q, want %q", got, want)
		}
	}
	unary := []UnaryOp{LogicNot, Pos, Neg, BitNot}
	for _, op := range unary {
		result, err := EvalUnary(op, Const{})
		if result.IsValid() {
			t.Errorf("EvalUnary got valid result %v, want invalid", result)
		}
		if got, want := fmt.Sprint(err), "invalid const"; got != want {
			t.Errorf("EvalUnary got error %q, want %q", got, want)
		}
	}
	binary := []BinaryOp{LogicAnd, LogicOr, EQ, NE, LT, LE, GT, GE, Add, Sub, Mul, Div, Mod, BitAnd, BitOr, BitXor, LeftShift, RightShift}
	for _, op := range binary {
		result, err := EvalBinary(op, Const{}, Const{})
		if result.IsValid() {
			t.Errorf("EvalBinary got valid result %v, want invalid", result)
		}
		if got, want := fmt.Sprint(err), "invalid const"; got != want {
			t.Errorf("EvalBinary got error %q, want %q", got, want)
		}
	}
}

func TestConstToValueOK(t *testing.T) {
	abcBytes := []byte("abc")
	tests := []*vdl.Value{
		vdl.BoolValue(nil, true), vdl.BoolValue(boolTypeN, true),
		vdl.StringValue(nil, "abc"), vdl.StringValue(stringTypeN, "abc"),
		vdl.BytesValue(nil, abcBytes), vdl.BytesValue(bytesTypeN, abcBytes),
		vdl.BytesValue(bytes3Type, abcBytes), vdl.BytesValue(bytes3TypeN, abcBytes),
		vdl.IntValue(vdl.Int32Type, 123), vdl.IntValue(int32TypeN, 123),
		vdl.UintValue(vdl.Uint32Type, 123), vdl.UintValue(uint32TypeN, 123),
		vdl.FloatValue(vdl.Float32Type, 123), vdl.FloatValue(float32TypeN, 123),
		structNumValue(structAIntType, sn{"A", 123}), structNumValue(structAIntTypeN, sn{"A", 123}),
	}
	for _, test := range tests {
		c := FromValue(test)
		v, err := c.ToValue()
		if got, want := v, test; !vdl.EqualValue(got, want) {
			t.Errorf("%v.ToValue got %v, want %v", c, got, want)
		}
		expectErr(t, err, "", "%v.ToValue", c)
	}
}

func TestConstToValueImplicit(t *testing.T) {
	tests := []struct {
		C Const
		V *vdl.Value
	}{
		{Boolean(true), vdl.BoolValue(nil, true)},
		{String("abc"), vdl.StringValue(nil, "abc")},
	}
	for _, test := range tests {
		c := FromValue(test.V)
		if got, want := c, test.C; !constEqual(got, want) {
			t.Errorf("FromValue(%v) got %v, want %v", test.C, got, want)
		}
		v, err := test.C.ToValue()
		if got, want := v, test.V; !vdl.EqualValue(got, want) {
			t.Errorf("%v.ToValue got %v, want %v", test.C, got, want)
		}
		expectErr(t, err, "", "%v.ToValue", test.C)
	}
}

func TestConstToValueError(t *testing.T) {
	tests := []struct {
		C      Const
		errstr string
	}{
		{Integer(bi1), noType},
		{Rational(br1), noType},
	}
	for _, test := range tests {
		v, err := test.C.ToValue()
		if v != nil {
			t.Errorf("%v.ToValue got %v, want nil", test.C, v)
		}
		expectErr(t, err, test.errstr, "%v.ToValue", test.C)
	}
}

type c []Const
type v []*vdl.Value

func TestConstConvertOK(t *testing.T) {
	abcBytes := []byte("abc")
	// Each test has a set of consts C and values V that are all convertible to
	// each other and equivalent.
	tests := []struct {
		C c
		V v
	}{
		{c{Boolean(true)},
			v{vdl.BoolValue(nil, true), vdl.BoolValue(boolTypeN, true)}},
		{c{String("abc")},
			v{vdl.StringValue(nil, "abc"), vdl.StringValue(stringTypeN, "abc"),
				vdl.BytesValue(nil, abcBytes), vdl.BytesValue(bytesTypeN, abcBytes),
				vdl.BytesValue(bytes3Type, abcBytes), vdl.BytesValue(bytes3TypeN, abcBytes)}},
		{c{Integer(bi1), Rational(br1)},
			v{vdl.IntValue(vdl.Int32Type, 1), vdl.IntValue(int32TypeN, 1),
				vdl.UintValue(vdl.Uint32Type, 1), vdl.UintValue(uint32TypeN, 1),
				vdl.FloatValue(vdl.Float32Type, 1), vdl.FloatValue(float32TypeN, 1)}},
		{c{Integer(biNeg1), Rational(brNeg1)},
			v{vdl.IntValue(vdl.Int32Type, -1), vdl.IntValue(int32TypeN, -1),
				vdl.FloatValue(vdl.Float32Type, -1), vdl.FloatValue(float32TypeN, -1)}},
		{c{Rational(big.NewRat(1, 2))},
			v{vdl.FloatValue(vdl.Float32Type, 0.5), vdl.FloatValue(float32TypeN, 0.5)}},
		// Check implicit conversion of untyped bool and string consts.
		{c{Boolean(true)},
			v{vdl.BoolValue(nil, true), vdl.AnyValue(vdl.BoolValue(nil, true))}},
		{c{String("abc")},
			v{vdl.StringValue(nil, "abc"), vdl.AnyValue(vdl.StringValue(nil, "abc"))}},
	}
	for _, test := range tests {
		// Create a slice of consts containing everything in C and V.
		consts := make([]Const, len(test.C))
		copy(consts, test.C)
		for _, v := range test.V {
			consts = append(consts, FromValue(v))
		}
		// Loop through the consts, and convert each one to each item in V.
		for _, c := range consts {
			for _, v := range test.V {
				vt := v.Type()
				got, err := c.Convert(vt)
				if want := FromValue(v); !constEqual(got, want) {
					t.Errorf("%v.Convert(%v) got %v, want %v", c, vt, got, want)
				}
				expectErr(t, err, "", "%v.Convert(%v)", c, vt)
			}
		}
	}
}

type ty []*vdl.Type

func TestConstConvertError(t *testing.T) {
	// Each test has a single const C that returns an error that contains errstr
	// when converted to any of the types in the set T.
	tests := []struct {
		C      Const
		T      ty
		errstr string
	}{
		{Boolean(true),
			ty{vdl.StringType, stringTypeN, bytesType, bytesTypeN, bytes3Type, bytes3TypeN,
				vdl.Int32Type, int32TypeN, vdl.Uint32Type, uint32TypeN,
				vdl.Float32Type, float32TypeN,
				structAIntType, structAIntTypeN},
			cantConvert},
		{String("abc"),
			ty{vdl.BoolType, boolTypeN,
				vdl.Int32Type, int32TypeN, vdl.Uint32Type, uint32TypeN,
				vdl.Float32Type, float32TypeN,
				structAIntType, structAIntTypeN},
			cantConvert},
		{Integer(bi1),
			ty{vdl.BoolType, boolTypeN,
				vdl.StringType, stringTypeN, bytesType, bytesTypeN, bytes3Type, bytes3TypeN,
				structAIntType, structAIntTypeN},
			cantConvert},
		{Rational(br1),
			ty{vdl.BoolType, boolTypeN,
				vdl.StringType, stringTypeN, bytesType, bytesTypeN, bytes3Type, bytes3TypeN,
				structAIntType, structAIntTypeN},
			cantConvert},
		// Bounds tests
		{Integer(biNeg1), ty{vdl.Uint32Type, uint32TypeN}, overflows},
		{Integer(big.NewInt(1 << 32)), ty{vdl.Int32Type, int32TypeN}, overflows},
		{Integer(big.NewInt(1 << 33)), ty{vdl.Uint32Type, uint32TypeN}, overflows},
		{Rational(brNeg1), ty{vdl.Uint32Type, uint32TypeN}, overflows},
		{Rational(big.NewRat(1<<32, 1)), ty{vdl.Int32Type, int32TypeN}, overflows},
		{Rational(big.NewRat(1<<33, 1)), ty{vdl.Uint32Type, uint32TypeN}, overflows},
		{Rational(big.NewRat(1, 2)),
			ty{vdl.Int32Type, int32TypeN, vdl.Uint32Type, uint32TypeN},
			losesPrecision},
		{Rational(bigRatAbsMin64), ty{vdl.Float32Type, float32TypeN}, underflows},
		{Rational(bigRatAbsMax64), ty{vdl.Float32Type, float32TypeN}, overflows},
	}
	for _, test := range tests {
		for _, ct := range test.T {
			result, err := test.C.Convert(ct)
			if result.IsValid() {
				t.Errorf("%v.Convert(%v) result got %v, want invalid", test.C, ct, result)
			}
			expectErr(t, err, test.errstr, "%v.Convert(%v)", test.C, ct)
		}
	}
}

func TestConstUnaryOpOK(t *testing.T) {
	tests := []struct {
		op        UnaryOp
		x, expect Const
	}{
		{LogicNot, Boolean(true), Boolean(false)},
		{LogicNot, boolConst(vdl.BoolType, false), boolConst(vdl.BoolType, true)},
		{LogicNot, boolConst(boolTypeN, true), boolConst(boolTypeN, false)},

		{Pos, Integer(bi1), Integer(bi1)},
		{Pos, Rational(br1), Rational(br1)},
		{Pos, intConst(vdl.Int32Type, 1), intConst(vdl.Int32Type, 1)},
		{Pos, floatConst(float32TypeN, 1), floatConst(float32TypeN, 1)},

		{Neg, Integer(bi1), Integer(biNeg1)},
		{Neg, Rational(br1), Rational(brNeg1)},
		{Neg, intConst(vdl.Int32Type, 1), intConst(vdl.Int32Type, -1)},
		{Neg, floatConst(float32TypeN, 1), floatConst(float32TypeN, -1)},

		{BitNot, Integer(bi1), Integer(biNeg2)},
		{BitNot, Rational(br1), Integer(biNeg2)},
		{BitNot, intConst(vdl.Int32Type, 1), intConst(vdl.Int32Type, -2)},
		{BitNot, uintConst(uint32TypeN, 1), uintConst(uint32TypeN, 1<<32-2)},
	}
	for _, test := range tests {
		result, err := EvalUnary(test.op, test.x)
		if got, want := result, test.expect; !constEqual(got, want) {
			t.Errorf("EvalUnary(%v, %v) result got %v, want %v", test.op, test.x, got, want)
		}
		expectErr(t, err, "", "EvalUnary(%v, %v)", test.op, test.x)
	}
}

func TestConstUnaryOpError(t *testing.T) {
	tests := []struct {
		op     UnaryOp
		x      Const
		errstr string
	}{
		{LogicNot, String("abc"), notSupported},
		{LogicNot, Integer(bi1), notSupported},
		{LogicNot, Rational(br1), notSupported},
		{LogicNot, structNumConst(structAIntTypeN, 999), notSupported},

		{Pos, Boolean(false), notSupported},
		{Pos, String("abc"), notSupported},
		{Pos, structNumConst(structAIntTypeN, 999), notSupported},

		{Neg, Boolean(false), notSupported},
		{Neg, String("abc"), notSupported},
		{Neg, structNumConst(structAIntTypeN, 999), notSupported},
		{Neg, intConst(vdl.Int32Type, -1<<31), overflows},

		{BitNot, Boolean(false), cantConvert},
		{BitNot, String("abc"), cantConvert},
		{BitNot, Rational(big.NewRat(1, 2)), losesPrecision},
		{BitNot, structNumConst(structAIntTypeN, 999), notSupported},
		{BitNot, floatConst(float32TypeN, 1), cantConvert},
	}
	for _, test := range tests {
		result, err := EvalUnary(test.op, test.x)
		if result.IsValid() {
			t.Errorf("EvalUnary(%v, %v) result got %v, want invalid", test.op, test.x, result)
		}
		expectErr(t, err, test.errstr, "EvalUnary(%v, %v)", test.op, test.x)
	}
}

func TestConstBinaryOpOK(t *testing.T) {
	tests := []struct {
		op           BinaryOp
		x, y, expect Const
	}{
		{LogicAnd, Boolean(true), Boolean(true), Boolean(true)},
		{LogicAnd, Boolean(true), Boolean(false), Boolean(false)},
		{LogicAnd, Boolean(false), Boolean(true), Boolean(false)},
		{LogicAnd, Boolean(false), Boolean(false), Boolean(false)},
		{LogicAnd, boolConst(boolTypeN, true), boolConst(boolTypeN, true), boolConst(boolTypeN, true)},
		{LogicAnd, boolConst(boolTypeN, true), boolConst(boolTypeN, false), boolConst(boolTypeN, false)},
		{LogicAnd, boolConst(boolTypeN, false), boolConst(boolTypeN, true), boolConst(boolTypeN, false)},
		{LogicAnd, boolConst(boolTypeN, false), boolConst(boolTypeN, false), boolConst(boolTypeN, false)},

		{LogicOr, Boolean(true), Boolean(true), Boolean(true)},
		{LogicOr, Boolean(true), Boolean(false), Boolean(true)},
		{LogicOr, Boolean(false), Boolean(true), Boolean(true)},
		{LogicOr, Boolean(false), Boolean(false), Boolean(false)},
		{LogicOr, boolConst(boolTypeN, true), boolConst(boolTypeN, true), boolConst(boolTypeN, true)},
		{LogicOr, boolConst(boolTypeN, true), boolConst(boolTypeN, false), boolConst(boolTypeN, true)},
		{LogicOr, boolConst(boolTypeN, false), boolConst(boolTypeN, true), boolConst(boolTypeN, true)},
		{LogicOr, boolConst(boolTypeN, false), boolConst(boolTypeN, false), boolConst(boolTypeN, false)},

		{Add, String("abc"), String("def"), String("abcdef")},
		{Add, Integer(bi1), Integer(bi1), Integer(bi2)},
		{Add, Rational(br1), Rational(br1), Rational(br2)},
		{Add, stringConst(stringTypeN, "abc"), stringConst(stringTypeN, "def"), stringConst(stringTypeN, "abcdef")},
		{Add, bytesConst(bytesTypeN, "abc"), bytesConst(bytesTypeN, "def"), bytesConst(bytesTypeN, "abcdef")},
		{Add, intConst(int32TypeN, 1), intConst(int32TypeN, 1), intConst(int32TypeN, 2)},
		{Add, uintConst(uint32TypeN, 1), uintConst(uint32TypeN, 1), uintConst(uint32TypeN, 2)},
		{Add, floatConst(float32TypeN, 1), floatConst(float32TypeN, 1), floatConst(float32TypeN, 2)},

		{Sub, Integer(bi2), Integer(bi1), Integer(bi1)},
		{Sub, Rational(br2), Rational(br1), Rational(br1)},
		{Sub, intConst(int32TypeN, 2), intConst(int32TypeN, 1), intConst(int32TypeN, 1)},
		{Sub, uintConst(uint32TypeN, 2), uintConst(uint32TypeN, 1), uintConst(uint32TypeN, 1)},
		{Sub, floatConst(float32TypeN, 2), floatConst(float32TypeN, 1), floatConst(float32TypeN, 1)},

		{Mul, Integer(bi2), Integer(bi2), Integer(bi4)},
		{Mul, Rational(br2), Rational(br2), Rational(br4)},
		{Mul, intConst(int32TypeN, 2), intConst(int32TypeN, 2), intConst(int32TypeN, 4)},
		{Mul, uintConst(uint32TypeN, 2), uintConst(uint32TypeN, 2), uintConst(uint32TypeN, 4)},
		{Mul, floatConst(float32TypeN, 2), floatConst(float32TypeN, 2), floatConst(float32TypeN, 4)},

		{Div, Integer(bi4), Integer(bi2), Integer(bi2)},
		{Div, Rational(br4), Rational(br2), Rational(br2)},
		{Div, intConst(int32TypeN, 4), intConst(int32TypeN, 2), intConst(int32TypeN, 2)},
		{Div, uintConst(uint32TypeN, 4), uintConst(uint32TypeN, 2), uintConst(uint32TypeN, 2)},
		{Div, floatConst(float32TypeN, 4), floatConst(float32TypeN, 2), floatConst(float32TypeN, 2)},

		{Mod, Integer(bi3), Integer(bi2), Integer(bi1)},
		{Mod, Rational(br3), Rational(br2), Rational(br1)},
		{Mod, intConst(int32TypeN, 3), intConst(int32TypeN, 2), intConst(int32TypeN, 1)},
		{Mod, uintConst(uint32TypeN, 3), uintConst(uint32TypeN, 2), uintConst(uint32TypeN, 1)},

		{BitAnd, Integer(bi3), Integer(bi2), Integer(bi2)},
		{BitAnd, Rational(br3), Rational(br2), Rational(br2)},
		{BitAnd, intConst(int32TypeN, 3), intConst(int32TypeN, 2), intConst(int32TypeN, 2)},
		{BitAnd, uintConst(uint32TypeN, 3), uintConst(uint32TypeN, 2), uintConst(uint32TypeN, 2)},

		{BitOr, Integer(bi5), Integer(bi3), Integer(bi7)},
		{BitOr, Rational(br5), Rational(br3), Rational(br7)},
		{BitOr, intConst(int32TypeN, 5), intConst(int32TypeN, 3), intConst(int32TypeN, 7)},
		{BitOr, uintConst(uint32TypeN, 5), uintConst(uint32TypeN, 3), uintConst(uint32TypeN, 7)},

		{BitXor, Integer(bi5), Integer(bi3), Integer(bi6)},
		{BitXor, Rational(br5), Rational(br3), Rational(br6)},
		{BitXor, intConst(int32TypeN, 5), intConst(int32TypeN, 3), intConst(int32TypeN, 6)},
		{BitXor, uintConst(uint32TypeN, 5), uintConst(uint32TypeN, 3), uintConst(uint32TypeN, 6)},

		{LeftShift, Integer(bi3), Integer(bi1), Integer(bi6)},
		{LeftShift, Rational(br3), Rational(br1), Rational(br6)},
		{LeftShift, intConst(int32TypeN, 3), intConst(int32TypeN, 1), intConst(int32TypeN, 6)},
		{LeftShift, uintConst(uint32TypeN, 3), uintConst(uint32TypeN, 1), uintConst(uint32TypeN, 6)},

		{RightShift, Integer(bi5), Integer(bi1), Integer(bi2)},
		{RightShift, Rational(br5), Rational(br1), Rational(br2)},
		{RightShift, intConst(int32TypeN, 5), intConst(int32TypeN, 1), intConst(int32TypeN, 2)},
		{RightShift, uintConst(uint32TypeN, 5), uintConst(uint32TypeN, 1), uintConst(uint32TypeN, 2)},
	}
	for _, test := range tests {
		result, err := EvalBinary(test.op, test.x, test.y)
		if got, want := result, test.expect; !constEqual(got, want) {
			t.Errorf("EvalBinary(%v, %v, %v) result got %v, want %v", test.op, test.x, test.y, got, want)
		}
		expectErr(t, err, "", "EvalBinary(%v, %v, %v)", test.op, test.x, test.y)
	}
}

func expectComp(t *testing.T, op BinaryOp, x, y Const, expect bool) {
	result, err := EvalBinary(op, x, y)
	if got, want := result, Boolean(expect); !constEqual(got, want) {
		t.Errorf("EvalBinary(%v, %v, %v) result got %v, want %v", op, x, y, got, want)
	}
	expectErr(t, err, "", "EvalBinary(%v, %v, %v)", op, x, y)
}

func TestConstEQNE(t *testing.T) {
	tests := []struct {
		x, y Const // x != y
	}{
		{Boolean(false), Boolean(true)},
		{String("abc"), String("def")},

		{boolConst(boolTypeN, false), boolConst(boolTypeN, true)},
		{structNumConst(structAIntTypeN, 1), structNumConst(structAIntTypeN, 2)},
	}
	for _, test := range tests {
		expectComp(t, EQ, test.x, test.x, true)
		expectComp(t, EQ, test.x, test.y, false)
		expectComp(t, EQ, test.y, test.x, false)
		expectComp(t, EQ, test.y, test.y, true)

		expectComp(t, NE, test.x, test.x, false)
		expectComp(t, NE, test.x, test.y, true)
		expectComp(t, NE, test.y, test.x, true)
		expectComp(t, NE, test.y, test.y, false)
	}
}

func TestConstOrdered(t *testing.T) {
	tests := []struct {
		x, y Const // x < y
	}{
		{String("abc"), String("def")},
		{Integer(bi1), Integer(bi2)},
		{Rational(br1), Rational(br2)},

		{stringConst(stringTypeN, "abc"), stringConst(stringTypeN, "def")},
		{bytesConst(bytesTypeN, "abc"), bytesConst(bytesTypeN, "def")},
		{bytesConst(bytes3TypeN, "abc"), bytesConst(bytes3TypeN, "def")},
		{intConst(int32TypeN, 1), intConst(int32TypeN, 2)},
		{uintConst(uint32TypeN, 1), uintConst(uint32TypeN, 2)},
		{floatConst(float32TypeN, 1), floatConst(float32TypeN, 2)},
	}
	for _, test := range tests {
		expectComp(t, EQ, test.x, test.x, true)
		expectComp(t, EQ, test.x, test.y, false)
		expectComp(t, EQ, test.y, test.x, false)
		expectComp(t, EQ, test.y, test.y, true)

		expectComp(t, NE, test.x, test.x, false)
		expectComp(t, NE, test.x, test.y, true)
		expectComp(t, NE, test.y, test.x, true)
		expectComp(t, NE, test.y, test.y, false)

		expectComp(t, LT, test.x, test.x, false)
		expectComp(t, LT, test.x, test.y, true)
		expectComp(t, LT, test.y, test.x, false)
		expectComp(t, LT, test.y, test.y, false)

		expectComp(t, LE, test.x, test.x, true)
		expectComp(t, LE, test.x, test.y, true)
		expectComp(t, LE, test.y, test.x, false)
		expectComp(t, LE, test.y, test.y, true)

		expectComp(t, GT, test.x, test.x, false)
		expectComp(t, GT, test.x, test.y, false)
		expectComp(t, GT, test.y, test.x, true)
		expectComp(t, GT, test.y, test.y, false)

		expectComp(t, GE, test.x, test.x, true)
		expectComp(t, GE, test.x, test.y, false)
		expectComp(t, GE, test.y, test.x, true)
		expectComp(t, GE, test.y, test.y, true)
	}
}

type bo []BinaryOp

func TestConstBinaryOpError(t *testing.T) {
	// For each op in Bops and each x in C, (x op x) returns errstr.
	tests := []struct {
		Bops   bo
		C      c
		errstr string
	}{
		// Type not supported / can't convert errors
		{bo{LogicAnd, LogicOr},
			c{String("abc"),
				stringConst(stringTypeN, "abc"),
				bytesConst(bytesTypeN, "abc"), bytesConst(bytes3TypeN, "abc"),
				Integer(bi1), intConst(int32TypeN, 1), uintConst(uint32TypeN, 1),
				Rational(br1), floatConst(float32TypeN, 1),
				structNumConst(structAIntType, 1), structNumConst(structAIntTypeN, 1)},
			notSupported},
		{bo{LT, LE, GT, GE},
			c{Boolean(true), boolConst(boolTypeN, false),
				structNumConst(structAIntType, 1), structNumConst(structAIntTypeN, 1)},
			notSupported},
		{bo{Add},
			c{structNumConst(structAIntType, 1), structNumConst(structAIntTypeN, 1)},
			notSupported},
		{bo{Sub, Mul, Div},
			c{String("abc"), stringConst(stringTypeN, "abc"),
				bytesConst(bytesTypeN, "abc"), bytesConst(bytes3TypeN, "abc"),
				structNumConst(structAIntType, 1), structNumConst(structAIntTypeN, 1)},
			notSupported},
		{bo{Mod, BitAnd, BitOr, BitXor, LeftShift, RightShift},
			c{String("abc"), stringConst(stringTypeN, "abc"),
				bytesConst(bytesTypeN, "abc"), bytesConst(bytes3TypeN, "abc"),
				structNumConst(structAIntType, 1), structNumConst(structAIntTypeN, 1)},
			cantConvert},
		// Bounds checking
		{bo{Add}, c{uintConst(uint32TypeN, 1<<31)}, overflows},
		{bo{Mul}, c{uintConst(uint32TypeN, 1<<16)}, overflows},
		{bo{Div}, c{uintConst(uint32TypeN, 0)}, divByZero},
		{bo{LeftShift}, c{uintConst(uint32TypeN, 32)}, overflows},
	}
	for _, test := range tests {
		for _, op := range test.Bops {
			for _, c := range test.C {
				result, err := EvalBinary(op, c, c)
				if result.IsValid() {
					t.Errorf("EvalBinary(%v, %v, %v) result got %v, want invalid", op, c, c, result)
				}
				expectErr(t, err, test.errstr, "EvalBinary(%v, %v, %v)", op, c, c)
			}
		}
	}
}
