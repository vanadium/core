// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package opconst defines the representation and operations for VDL constants.
package opconst

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"strconv"

	"v.io/v23/vdl"
)

var (
	bigIntZero     = new(big.Int)
	bigRatZero     = new(big.Rat)
	bigIntOne      = big.NewInt(1)
	bigRatAbsMin32 = new(big.Rat).SetFloat64(math.SmallestNonzeroFloat32)
	bigRatAbsMax32 = new(big.Rat).SetFloat64(math.MaxFloat32)
	bigRatAbsMin64 = new(big.Rat).SetFloat64(math.SmallestNonzeroFloat64)
	bigRatAbsMax64 = new(big.Rat).SetFloat64(math.MaxFloat64)
	maxShiftSize   = big.NewInt(2000) // arbitrary large value

	errInvalidConst = errors.New("invalid const")
	errConvertNil   = errors.New("invalid conversion to untyped const")
	errDivZero      = errors.New("divide by zero")
)

// Const represents a constant value, similar in spirit to Go constants.  Consts
// may be typed or untyped.  Typed consts represent unchanging Values; all
// Values may be converted into valid typed consts, and all typed consts may be
// converted into valid Values.  Untyped consts belong to one of the following
// categories:
//   untyped boolean
//   untyped string
//   untyped integer
//   untyped rational
// Literal consts are untyped, as are expressions only containing untyped
// consts.  The result of comparison operations is untyped boolean.
//
// Operations are represented by UnaryOp and BinaryOp, and are supported on
// Consts, but not Values.  We support common logical, bitwise, comparison and
// arithmetic operations.  Not all operations are supported on all consts.
//
// Binary ops where both sides are typed consts return errors on type
// mismatches; e.g. uint32(1) + uint64(1) is an invalid binary add.  Ops on
// typed consts also return errors on loss of precision; e.g. uint32(1.1)
// returns an error.
//
// Binary ops where one or both sides are untyped consts perform implicit type
// conversion.  E.g. uint32(1) + 1 is a valid binary add, where the
// right-hand-side is the untyped integer const 1, which is coerced to the
// uint32 type before the op is performed.  Operations only containing untyped
// consts are performed with "infinite" precision.
//
// The zero Const is invalid.
type Const struct {
	// rep holds the underlying representation, it may be one of:
	//   bool        - Represents typed and untyped boolean constants.
	//   string      - Represents typed and untyped string constants.
	//   *big.Int    - Represents typed and untyped integer constants.
	//   *big.Rat    - Represents typed and untyped rational constants.
	//   *Value      - Represents all other typed constants.
	rep interface{}

	// repType holds the type of rep.  If repType is nil the constant is untyped,
	// otherwise the constant is typed, and rep must match the kind of repType.
	// If rep is a *Value, repType is always non-nil.
	repType *vdl.Type
}

// Boolean returns an untyped boolean Const.
func Boolean(x bool) Const { return Const{x, nil} }

// String returns an untyped string Const.
func String(x string) Const { return Const{x, nil} }

// Integer returns an untyped integer Const.
func Integer(x *big.Int) Const { return Const{x, nil} }

// Rational returns an untyped rational Const.
func Rational(x *big.Rat) Const { return Const{x, nil} }

// TODO(toddw): Use big.Float to represent floating point, rather than big.Rat.
// We'll still use big.Rat to represent rationals, e.g. integer division.

// FromValue returns a typed Const based on value v.
func FromValue(v *vdl.Value) Const {
	if v.Type().IsBytes() {
		// Represent []byte and [N]byte as a string, so that conversions are easy.
		return Const{string(v.Bytes()), v.Type()}
	}
	switch v.Kind() {
	case vdl.Bool:
		if v.Type() == vdl.BoolType { // Treat unnamed bool as untyped bool.
			return Boolean(v.Bool())
		}
		return Const{v.Bool(), v.Type()}
	case vdl.String:
		if v.Type() == vdl.StringType { // Treat unnamed string as untyped string.
			return String(v.RawString())
		}
		return Const{v.RawString(), v.Type()}
	case vdl.Byte, vdl.Uint16, vdl.Uint32, vdl.Uint64:
		return Const{new(big.Int).SetUint64(v.Uint()), v.Type()}
	case vdl.Int8, vdl.Int16, vdl.Int32, vdl.Int64:
		return Const{new(big.Int).SetInt64(v.Int()), v.Type()}
	case vdl.Float32, vdl.Float64:
		return Const{new(big.Rat).SetFloat64(v.Float()), v.Type()}
	default:
		return Const{v, v.Type()}
	}
}

// IsValid returns true iff the c represents a const; it returns false for the
// zero Const.
func (c Const) IsValid() bool {
	return c.rep != nil
}

// Type returns the type of c.  Nil indicates c is an untyped const.
func (c Const) Type() *vdl.Type {
	return c.repType
}

// Convert converts c to the target type t, and returns the resulting const.
// Returns an error if t is nil; you're not allowed to convert into an untyped
// const.
func (c Const) Convert(t *vdl.Type) (Const, error) {
	if t == nil {
		return Const{}, errConvertNil
	}
	// If we're trying to convert to Any or Union, or if c is already a vdl.Value,
	// use vdl.Convert to convert as a vdl.Value.
	_, isValue := c.rep.(*vdl.Value)
	if isValue || t.Kind() == vdl.Any || t.Kind() == vdl.Union {
		src, err := c.ToValue()
		if err != nil {
			return Const{}, err
		}
		dst := vdl.ZeroValue(t)
		if err := vdl.Convert(dst, src); err != nil {
			return Const{}, err
		}
		return FromValue(dst), nil
	}
	// Otherwise use makeConst to convert as a Const.
	return makeConst(c.rep, t)
}

func (c Const) String() string {
	if !c.IsValid() {
		return "invalid"
	}
	if v, ok := c.rep.(*vdl.Value); ok {
		return v.String()
	}
	if c.repType == nil {
		// E.g. 12345
		return cRepString(c.rep)
	}
	// E.g. int32(12345)
	return c.typeString() + "(" + cRepString(c.rep) + ")"
}

func (c Const) typeString() string {
	return cRepTypeString(c.rep, c.repType)
}

// cRepString returns a human-readable string representing the const value.
func cRepString(rep interface{}) string {
	switch trep := rep.(type) {
	case nil:
		return "" // invalid const
	case bool:
		if trep {
			return "true"
		}
		return "false"
	case string:
		return strconv.Quote(trep)
	case *big.Int:
		return trep.String()
	case *big.Rat:
		if trep.IsInt() {
			return trep.Num().String() + ".0"
		}
		frep, _ := trep.Float64()
		return strconv.FormatFloat(frep, 'g', -1, 64)
	case *vdl.Value:
		return trep.String()
	default:
		panic(fmt.Errorf("val: unhandled const type %T value %v", rep, rep))
	}
}

// cRepTypeString returns a human-readable string representing the type of
// the const value.
func cRepTypeString(rep interface{}, t *vdl.Type) string {
	if t != nil {
		return t.String()
	}
	switch rep.(type) {
	case nil:
		return "invalid"
	case bool:
		return "untyped boolean"
	case string:
		return "untyped string"
	case *big.Int:
		return "untyped integer"
	case *big.Rat:
		return "untyped rational"
	default:
		panic(fmt.Errorf("val: unhandled const type %T value %v", rep, rep))
	}
}

// ToValue converts Const c to a Value.
func (c Const) ToValue() (*vdl.Value, error) { //nolint:gocyclo
	if c.rep == nil {
		return nil, errInvalidConst
	}
	// All const defs must have a type.  We implicitly assign bool and string, but
	// the user must explicitly assign a type for numeric consts.
	if c.repType == nil {
		switch c.rep.(type) {
		case bool:
			c.repType = vdl.BoolType
		case string:
			c.repType = vdl.StringType
		default:
			return nil, fmt.Errorf("%s must be assigned a type", c)
		}
	}
	// Create a value of the appropriate type.
	vx := vdl.ZeroValue(c.repType)
	switch trep := c.rep.(type) {
	case bool:
		if vx.Kind() == vdl.Bool {
			vx.AssignBool(trep)
			return vx, nil
		}
	case string:
		switch {
		case vx.Kind() == vdl.String:
			vx.AssignString(trep)
			return vx, nil
		case vx.Type().IsBytes():
			if vx.Kind() == vdl.Array {
				if vx.Len() != len(trep) {
					return nil, fmt.Errorf("%s has a different length than %v", c, vx.Type())
				}
			}
			vx.AssignBytes([]byte(trep))
			return vx, nil
		}
	case *big.Int:
		switch vx.Kind() {
		case vdl.Byte, vdl.Uint16, vdl.Uint32, vdl.Uint64:
			vx.AssignUint(trep.Uint64())
			return vx, nil
		case vdl.Int8, vdl.Int16, vdl.Int32, vdl.Int64:
			vx.AssignInt(trep.Int64())
			return vx, nil
		}
	case *big.Rat:
		switch vx.Kind() {
		case vdl.Float32, vdl.Float64:
			f64, _ := trep.Float64()
			vx.AssignFloat(f64)
			return vx, nil
		}
	case *vdl.Value:
		return trep, nil
	}
	// Type mismatches shouldn't occur, since makeConst always ensures the rep and
	// repType are in sync.  If something's wrong we want to know about it.
	panic(fmt.Errorf("val: mismatched const rep type for %v", c))
}

func errNotSupported(rep interface{}, t *vdl.Type) error {
	return fmt.Errorf("%s not supported", cRepTypeString(rep, t))
}

// EvalUnary returns the result of evaluating (op x).
func EvalUnary(op UnaryOp, x Const) (Const, error) { //nolint:gocyclo
	if x.rep == nil {
		return Const{}, errInvalidConst
	}
	if _, ok := x.rep.(*vdl.Value); ok {
		// There are no valid unary ops on *Value consts.
		return Const{}, errNotSupported(x.rep, x.repType)
	}
	switch op {
	case LogicNot:
		if tx, ok := x.rep.(bool); ok {
			return makeConst(!tx, x.repType)
		}
	case Pos:
		switch x.rep.(type) {
		case *big.Int, *big.Rat:
			return x, nil
		}
	case Neg:
		switch tx := x.rep.(type) {
		case *big.Int:
			return makeConst(new(big.Int).Neg(tx), x.repType)
		case *big.Rat:
			return makeConst(new(big.Rat).Neg(tx), x.repType)
		}
	case BitNot:
		ix, err := constToInt(x)
		if err != nil {
			return Const{}, err
		}
		// big.Int.Not implements bit-not for signed integers, but we need to
		// special-case unsigned integers.  E.g. ^int8(1)=-2, ^uint8(1)=254
		not := new(big.Int)
		switch {
		case x.repType != nil && x.repType.Kind() == vdl.Byte:
			not.SetUint64(uint64(^uint8(ix.Uint64())))
		case x.repType != nil && x.repType.Kind() == vdl.Uint16:
			not.SetUint64(uint64(^uint16(ix.Uint64())))
		case x.repType != nil && x.repType.Kind() == vdl.Uint32:
			not.SetUint64(uint64(^uint32(ix.Uint64())))
		case x.repType != nil && x.repType.Kind() == vdl.Uint64:
			not.SetUint64(^ix.Uint64())
		default:
			not.Not(ix)
		}
		return makeConst(not, x.repType)
	}
	return Const{}, errNotSupported(x.rep, x.repType)
}

// EvalBinary returns the result of evaluating (x op y).
func EvalBinary(op BinaryOp, x, y Const) (Const, error) {
	if x.rep == nil || y.rep == nil {
		return Const{}, errInvalidConst
	}
	switch op {
	case LeftShift, RightShift:
		// Shift ops are special since they require an integer lhs and unsigned rhs.
		return evalShift(op, x, y)
	}
	// All other binary ops behave similarly.  First we perform implicit
	// conversion of x and y.  If either side is untyped, we may need to
	// implicitly convert it to the type of the other side.  If both sides are
	// typed they need to match.  The resulting tx and ty are guaranteed to have
	// the same type, and resType tells us which type we need to convert the
	// result into when we're done.
	cx, cy, resType, err := coerceConsts(x, y)
	if err != nil {
		return Const{}, err
	}
	// Now we perform the actual binary op.
	var res interface{}
	switch op {
	case LogicOr, LogicAnd:
		res, err = opLogic(op, cx, cy, resType)
	case EQ, NE, LT, LE, GT, GE:
		res, err = opComp(op, cx, cy, resType)
		resType = nil // comparisons always result in untyped bool.
	case Add, Sub, Mul, Div:
		res, err = opArith(op, cx, cy, resType)
	case Mod, BitAnd, BitOr, BitXor:
		res, err = opIntArith(op, cx, cy, resType)
	default:
		err = errNotSupported(cx, resType)
	}
	if err != nil {
		return Const{}, err
	}
	// As a final step we convert to the result type.
	return makeConst(res, resType)
}

func opLogic(op BinaryOp, x, y interface{}, resType *vdl.Type) (interface{}, error) {
	if tx, ok := x.(bool); ok {
		switch op {
		case LogicOr:
			return tx || y.(bool), nil
		case LogicAnd:
			return tx && y.(bool), nil
		}
	}
	return nil, errNotSupported(x, resType)
}

func opComp(op BinaryOp, x, y interface{}, resType *vdl.Type) (interface{}, error) {
	switch tx := x.(type) {
	case bool:
		switch op {
		case EQ:
			return tx == y.(bool), nil
		case NE:
			return tx != y.(bool), nil
		}
	case string:
		return compString(op, tx, y.(string)), nil
	case *big.Int:
		return opCmpToBool(op, tx.Cmp(y.(*big.Int))), nil
	case *big.Rat:
		return opCmpToBool(op, tx.Cmp(y.(*big.Rat))), nil
	case *vdl.Value:
		switch op {
		case EQ:
			return vdl.EqualValue(tx, y.(*vdl.Value)), nil
		case NE:
			return !vdl.EqualValue(tx, y.(*vdl.Value)), nil
		}
	}
	return nil, errNotSupported(x, resType)
}

func opArith(op BinaryOp, x, y interface{}, resType *vdl.Type) (interface{}, error) {
	switch tx := x.(type) {
	case string:
		if op == Add {
			return tx + y.(string), nil
		}
	case *big.Int:
		return arithBigInt(op, tx, y.(*big.Int))
	case *big.Rat:
		return arithBigRat(op, tx, y.(*big.Rat))
	}
	return nil, errNotSupported(x, resType)
}

func opIntArith(op BinaryOp, x, y interface{}, resType *vdl.Type) (interface{}, error) {
	ix, err := constToInt(Const{x, resType})
	if err != nil {
		return nil, err
	}
	iy, err := constToInt(Const{y, resType})
	if err != nil {
		return nil, err
	}
	return arithBigInt(op, ix, iy)
}

func evalShift(op BinaryOp, x, y Const) (Const, error) {
	// lhs must be an integer.
	ix, err := constToInt(x)
	if err != nil {
		return Const{}, err
	}
	// rhs must be a small unsigned integer.
	iy, err := constToInt(y)
	if err != nil {
		return Const{}, err
	}
	if iy.Sign() < 0 {
		return Const{}, fmt.Errorf("shift amount %v isn't unsigned", cRepString(iy))
	}
	if iy.Cmp(maxShiftSize) > 0 {
		return Const{}, fmt.Errorf("shift amount %v greater than max allowed %v", cRepString(iy), cRepString(maxShiftSize))
	}
	// Perform the shift and convert it back to the lhs type.
	return makeConst(shiftBigInt(op, ix, uint(iy.Uint64())), x.repType)
}

// bigRatToInt converts rational to integer values as long as there isn't any
// loss in precision, checking resType to make sure the conversion is allowed.
func bigRatToInt(rat *big.Rat, resType *vdl.Type) (*big.Int, error) {
	// As a special-case we allow untyped rat consts to be converted to integers,
	// as long as they can do so without loss of precision.  This is safe since
	// untyped rat consts have "unbounded" precision.  Typed float consts may have
	// been rounded at some point, so we don't allow this.  This is the same
	// behavior as Go.
	if resType != nil {
		return nil, fmt.Errorf("can't convert typed %s to integer", cRepTypeString(rat, resType))
	}
	if !rat.IsInt() {
		return nil, fmt.Errorf("converting %s %s to integer loses precision", cRepTypeString(rat, resType), cRepString(rat))
	}
	return new(big.Int).Set(rat.Num()), nil
}

// constToInt converts x to an integer value as long as there isn't any loss in
// precision.
func constToInt(x Const) (*big.Int, error) {
	switch tx := x.rep.(type) {
	case *big.Int:
		return tx, nil
	case *big.Rat:
		return bigRatToInt(tx, x.repType)
	}
	return nil, fmt.Errorf("can't convert %s to integer", x.typeString())
}

// makeConst creates a Const with value rep and type totype, performing overflow
// and conversion checks on numeric values.  If totype is nil the resulting
// const is untyped.
//
// TODO(toddw): Update to handle conversions to optional types.
func makeConst(rep interface{}, totype *vdl.Type) (Const, error) { //nolint:gocyclo
	if rep == nil {
		return Const{}, errInvalidConst
	}
	if totype == nil {
		if v, ok := rep.(*vdl.Value); ok {
			return Const{}, fmt.Errorf("can't make typed value %s untyped", v.Type())
		}
		return Const{rep, nil}, nil
	}
	switch trep := rep.(type) {
	case bool:
		if totype.Kind() == vdl.Bool {
			return Const{trep, totype}, nil
		}
	case string:
		if totype.Kind() == vdl.String || totype.IsBytes() {
			return Const{trep, totype}, nil
		}
	case *big.Int:
		switch totype.Kind() {
		case vdl.Byte, vdl.Uint16, vdl.Uint32, vdl.Uint64, vdl.Int8, vdl.Int16, vdl.Int32, vdl.Int64:
			if err := checkOverflowInt(trep, totype.Kind()); err != nil {
				return Const{}, err
			}
			return Const{trep, totype}, nil
		case vdl.Float32, vdl.Float64:
			return makeConst(new(big.Rat).SetInt(trep), totype)
		}
	case *big.Rat:
		switch totype.Kind() {
		case vdl.Byte, vdl.Uint16, vdl.Uint32, vdl.Uint64, vdl.Int8, vdl.Int16, vdl.Int32, vdl.Int64:
			// The only way we reach this conversion from big.Rat to a typed integer
			// is for explicit type conversions.  We pass a nil Type to bigRatToInt
			// indicating trep is untyped, to allow all conversions from float to int
			// as long as trep is actually an integer.
			irep, err := bigRatToInt(trep, nil)
			if err != nil {
				return Const{}, err
			}
			return makeConst(irep, totype)
		case vdl.Float32, vdl.Float64:
			frep, err := convertTypedRat(trep, totype.Kind())
			if err != nil {
				return Const{}, err
			}
			return Const{frep, totype}, nil
		}
	}
	return Const{}, fmt.Errorf("can't convert %s to %v", cRepString(rep), cRepTypeString(rep, totype))
}

func bitLenInt(kind vdl.Kind) int {
	switch kind {
	case vdl.Byte, vdl.Int8:
		return 8
	case vdl.Uint16, vdl.Int16:
		return 16
	case vdl.Uint32, vdl.Int32:
		return 32
	case vdl.Uint64, vdl.Int64:
		return 64
	default:
		panic(fmt.Errorf("val: bitLen unhandled kind %v", kind))
	}
}

// checkOverflowInt returns an error iff converting b to the typed integer will
// cause overflow.
func checkOverflowInt(b *big.Int, kind vdl.Kind) error {
	switch bitlen := bitLenInt(kind); kind {
	case vdl.Byte, vdl.Uint16, vdl.Uint32, vdl.Uint64:
		if b.Sign() < 0 || b.BitLen() > bitlen {
			return fmt.Errorf("const %v overflows uint%d", cRepString(b), bitlen)
		}
	case vdl.Int8, vdl.Int16, vdl.Int32, vdl.Int64:
		// Account for two's complement, where e.g. int8 ranges from -128 to 127
		if b.Sign() >= 0 {
			// Positives and 0 - just check bitlen, accounting for the sign bit.
			if b.BitLen() >= bitlen {
				return fmt.Errorf("const %v overflows int%d", cRepString(b), bitlen)
			}
		} else {
			// Negatives need to take an extra value into account (e.g. -128 for int8)
			bplus1 := new(big.Int).Add(b, bigIntOne)
			if bplus1.BitLen() >= bitlen {
				return fmt.Errorf("const %v overflows int%d", cRepString(b), bitlen)
			}
		}
	default:
		panic(fmt.Errorf("val: checkOverflowInt unhandled kind %v", kind))
	}
	return nil
}

// checkOverflowRat returns an error iff converting b to the typed rat will
// cause overflow or underflow.
func checkOverflowRat(b *big.Rat, kind vdl.Kind) error {
	// Exact zero is special cased in ieee754.
	if b.Cmp(bigRatZero) == 0 {
		return nil
	}
	// TODO(toddw): perhaps allow slightly smaller and larger values, to account
	// for ieee754 round-to-even rules.
	switch abs := new(big.Rat).Abs(b); kind {
	case vdl.Float32:
		if abs.Cmp(bigRatAbsMin32) < 0 {
			return fmt.Errorf("const %v underflows float32", cRepString(b))
		}
		if abs.Cmp(bigRatAbsMax32) > 0 {
			return fmt.Errorf("const %v overflows float32", cRepString(b))
		}
	case vdl.Float64:
		if abs.Cmp(bigRatAbsMin64) < 0 {
			return fmt.Errorf("const %v underflows float64", cRepString(b))
		}
		if abs.Cmp(bigRatAbsMax64) > 0 {
			return fmt.Errorf("const %v overflows float64", cRepString(b))
		}
	default:
		panic(fmt.Errorf("val: checkOverflowRat unhandled kind %v", kind))
	}
	return nil
}

// convertTypedRat converts b to the typed rat, rounding as necessary.
func convertTypedRat(b *big.Rat, kind vdl.Kind) (*big.Rat, error) {
	if err := checkOverflowRat(b, kind); err != nil {
		return nil, err
	}
	switch f64, _ := b.Float64(); kind {
	case vdl.Float32:
		return new(big.Rat).SetFloat64(float64(float32(f64))), nil
	case vdl.Float64:
		return new(big.Rat).SetFloat64(f64), nil
	default:
		panic(fmt.Errorf("val: convertTypedRat unhandled kind %v", kind))
	}
}

// coerceConsts performs implicit conversion of cl and cr based on their
// respective types.  Returns the converted values vl and vr which are
// guaranteed to be of the same type represented by the returned Type, which may
// be nil if both consts are untyped.
func coerceConsts(cl, cr Const) (interface{}, interface{}, *vdl.Type, error) { //nolint:gocyclo
	var err error
	if cl.repType != nil && cr.repType != nil {
		// Both consts are typed - their types must match (no implicit conversion).
		if cl.repType != cr.repType {
			return nil, nil, nil, fmt.Errorf("type mismatch %v and %v", cl.typeString(), cr.typeString())
		}
		return cl.rep, cr.rep, cl.repType, nil
	}
	if cl.repType != nil {
		// Convert rhs to the type of the lhs.
		cr, err = makeConst(cr.rep, cl.repType)
		if err != nil {
			return nil, nil, nil, err
		}
		return cl.rep, cr.rep, cl.repType, nil
	}
	if cr.repType != nil {
		// Convert lhs to the type of the rhs.
		cl, err = makeConst(cl.rep, cr.repType)
		if err != nil {
			return nil, nil, nil, err
		}
		return cl.rep, cr.rep, cr.repType, nil
	}
	// Both consts are untyped, might need to implicitly promote untyped consts.
	switch vl := cl.rep.(type) {
	case bool:
		if vr, ok := cr.rep.(bool); ok {
			return vl, vr, nil, nil
		}
	case string:
		if vr, ok := cr.rep.(string); ok {
			return vl, vr, nil, nil
		}
	case *big.Int:
		switch vr := cr.rep.(type) {
		case *big.Int:
			return vl, vr, nil, nil
		case *big.Rat:
			// Promote lhs to rat
			return new(big.Rat).SetInt(vl), vr, nil, nil
		}
	case *big.Rat:
		switch vr := cr.rep.(type) {
		case *big.Int:
			// Promote rhs to rat
			return vl, new(big.Rat).SetInt(vr), nil, nil
		case *big.Rat:
			return vl, vr, nil, nil
		}
	}
	return nil, nil, nil, fmt.Errorf("mismatched %s and %s", cl.typeString(), cr.typeString())
}

func compString(op BinaryOp, l, r string) bool {
	switch op {
	case EQ:
		return l == r
	case NE:
		return l != r
	case LT:
		return l < r
	case LE:
		return l <= r
	case GT:
		return l > r
	case GE:
		return l >= r
	default:
		panic(fmt.Errorf("val: unhandled op %q", op))
	}
}

func opCmpToBool(op BinaryOp, cmp int) bool {
	switch op {
	case EQ:
		return cmp == 0
	case NE:
		return cmp != 0
	case LT:
		return cmp < 0
	case LE:
		return cmp <= 0
	case GT:
		return cmp > 0
	case GE:
		return cmp >= 0
	default:
		panic(fmt.Errorf("val: unhandled op %q", op))
	}
}

func arithBigInt(op BinaryOp, l, r *big.Int) (*big.Int, error) {
	switch op {
	case Add:
		return new(big.Int).Add(l, r), nil
	case Sub:
		return new(big.Int).Sub(l, r), nil
	case Mul:
		return new(big.Int).Mul(l, r), nil
	case Div:
		if r.Cmp(bigIntZero) == 0 {
			return nil, errDivZero
		}
		return new(big.Int).Quo(l, r), nil
	case Mod:
		if r.Cmp(bigIntZero) == 0 {
			return nil, errDivZero
		}
		return new(big.Int).Rem(l, r), nil
	case BitAnd:
		return new(big.Int).And(l, r), nil
	case BitOr:
		return new(big.Int).Or(l, r), nil
	case BitXor:
		return new(big.Int).Xor(l, r), nil
	default:
		panic(fmt.Errorf("val: unhandled op %q", op))
	}
}

func arithBigRat(op BinaryOp, l, r *big.Rat) (*big.Rat, error) {
	switch op {
	case Add:
		return new(big.Rat).Add(l, r), nil
	case Sub:
		return new(big.Rat).Sub(l, r), nil
	case Mul:
		return new(big.Rat).Mul(l, r), nil
	case Div:
		if r.Cmp(bigRatZero) == 0 {
			return nil, errDivZero
		}
		inv := new(big.Rat).Inv(r)
		return inv.Mul(inv, l), nil
	default:
		panic(fmt.Errorf("val: unhandled op %q", op))
	}
}

func shiftBigInt(op BinaryOp, l *big.Int, n uint) *big.Int {
	switch op {
	case LeftShift:
		return new(big.Int).Lsh(l, n)
	case RightShift:
		return new(big.Int).Rsh(l, n)
	default:
		panic(fmt.Errorf("val: unhandled op %q", op))
	}
}
