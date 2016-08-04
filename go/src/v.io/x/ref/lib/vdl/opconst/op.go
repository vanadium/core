// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package opconst

// UnaryOp represents a unary operation to be performed on a Const.
type UnaryOp uint

// BinaryOp represents a binary operation to be performed on two Consts.
type BinaryOp uint

const (
	InvalidUnaryOp UnaryOp = iota
	LogicNot               //  ! logical not
	Pos                    //  + positive (nop)
	Neg                    //  - negate
	BitNot                 //  ^ bitwise not
)

const (
	InvalidBinaryOp BinaryOp = iota
	LogicAnd                 //  && logical and
	LogicOr                  //  || logical or
	EQ                       //  == equal
	NE                       //  != not equal
	LT                       //  <  less than
	LE                       //  <= less than or equal
	GT                       //  >  greater than
	GE                       //  >= greater than or equal
	Add                      //  +  add
	Sub                      //  -  subtract
	Mul                      //  *  multiply
	Div                      //  /  divide
	Mod                      //  %  modulo
	BitAnd                   //  &  bitwise and
	BitOr                    //  |  bitwise or
	BitXor                   //  ^  bitwise xor
	LeftShift                //  << left shift
	RightShift               //  >> right shift
)

var unaryOpTable = [...]struct {
	symbol, desc string
}{
	InvalidUnaryOp: {"invalid", "invalid"},
	LogicNot:       {"!", "logic_not"},
	Pos:            {"+", "pos"},
	Neg:            {"-", "neg"},
	BitNot:         {"^", "bit_not"},
}

var binaryOpTable = [...]struct {
	symbol, desc string
}{
	InvalidBinaryOp: {"invalid", "invalid"},
	LogicAnd:        {"&&", "logic_and"},
	LogicOr:         {"||", "logic_or"},
	EQ:              {"==", "eq"},
	NE:              {"!=", "ne"},
	LT:              {"<", "lt"},
	LE:              {"<=", "le"},
	GT:              {">", "gt"},
	GE:              {">=", "ge"},
	Add:             {"+", "add"},
	Sub:             {"-", "sub"},
	Mul:             {"*", "mul"},
	Div:             {"/", "div"},
	Mod:             {"%", "mod"},
	BitAnd:          {"&", "bit_and"},
	BitOr:           {"|", "bit_or"},
	BitXor:          {"^", "bit_xor"},
	LeftShift:       {"<<", "left_shift"},
	RightShift:      {">>", "right_shift"},
}

func (op UnaryOp) String() string  { return unaryOpTable[op].desc }
func (op BinaryOp) String() string { return binaryOpTable[op].desc }

// ToUnaryOp converts s into a UnaryOp, or returns InvalidUnaryOp if it couldn't
// be converted.
func ToUnaryOp(s string) UnaryOp {
	for op, item := range unaryOpTable {
		if s == item.symbol || s == item.desc {
			return UnaryOp(op)
		}
	}
	return InvalidUnaryOp
}

// ToBinaryOp converts s into a BinaryOp, or returns InvalidBinaryOp if it
// couldn't be converted.
func ToBinaryOp(s string) BinaryOp {
	for op, item := range binaryOpTable {
		if s == item.symbol || s == item.desc {
			return BinaryOp(op)
		}
	}
	return InvalidBinaryOp
}
