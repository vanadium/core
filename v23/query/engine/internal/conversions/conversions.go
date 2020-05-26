// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conversions

import (
	"errors"
	"math/big"
	"strings"

	"v.io/v23/query/engine/internal/queryparser"
)

func ConvertValueToString(o *queryparser.Operand) (*queryparser.Operand, error) {
	// Other types must be explicitly converted to string with the Str() function.
	var c queryparser.Operand
	c.Type = queryparser.TypStr
	c.Off = o.Off
	switch o.Type {
	case queryparser.TypStr:
		c.Str = o.Str
		c.Prefix = o.Prefix   // non-nil for rhs of like expressions
		c.Pattern = o.Pattern // non-nil for rhs of like expressions
	default:
		return nil, errors.New("cannot convert operand to string")
	}
	return &c, nil
}

func ConvertValueToTime(o *queryparser.Operand) (*queryparser.Operand, error) {
	switch o.Type {
	case queryparser.TypTime:
		return o, nil
	default:
		return nil, errors.New("cannot convert operand to time")
	}
}

func ConvertValueToBigRat(o *queryparser.Operand) (*queryparser.Operand, error) {
	// operand cannot be string literal.
	var c queryparser.Operand
	c.Type = queryparser.TypBigRat
	switch o.Type {
	case queryparser.TypBigInt:
		var b big.Rat
		c.BigRat = b.SetInt(o.BigInt)
	case queryparser.TypBigRat:
		c.BigRat = o.BigRat
	case queryparser.TypBool:
		return nil, errors.New("cannot convert bool to big.Rat")
	case queryparser.TypFloat:
		var b big.Rat
		c.BigRat = b.SetFloat64(o.Float)
	case queryparser.TypInt:
		c.BigRat = big.NewRat(o.Int, 1)
	case queryparser.TypUint:
		var bi big.Int
		bi.SetUint64(o.Uint)
		var br big.Rat
		c.BigRat = br.SetInt(&bi)
	case queryparser.TypObject:
		return nil, errors.New("cannot convert object to big.Rat")
	default:
		// TODO(jkline): Log this logic error and all other similar cases.
		return nil, errors.New("cannot convert operand to big.Rat")
	}
	return &c, nil
}

func ConvertValueToFloat(o *queryparser.Operand) (*queryparser.Operand, error) {
	// Operand cannot be literal, big.Rat or big.Int
	var c queryparser.Operand
	c.Type = queryparser.TypFloat
	switch o.Type {
	case queryparser.TypBool:
		return nil, errors.New("cannot convert bool to float64")
	case queryparser.TypFloat:
		c.Float = o.Float
	case queryparser.TypInt:
		c.Float = float64(o.Int)
	case queryparser.TypUint:
		c.Float = float64(o.Uint)
	case queryparser.TypObject:
		return nil, errors.New("cannot convert object to float64")
	default:
		// TODO(jkline): Log this logic error and all other similar cases.
		return nil, errors.New("cannot convert operand to float64")
	}
	return &c, nil
}

func ConvertValueToBool(o *queryparser.Operand) (*queryparser.Operand, error) {
	var c queryparser.Operand
	c.Type = queryparser.TypBool
	switch o.Type {
	case queryparser.TypBool:
		c.Bool = o.Bool
	case queryparser.TypStr:
		switch strings.ToLower(o.Str) {
		case "true":
			c.Bool = true
		case "false":
			c.Bool = false
		default:
			return nil, errors.New("cannot convert object to bool")
		}
	default:
		return nil, errors.New("cannot convert operand to bool")
	}
	return &c, nil
}

func ConvertValueToBigInt(o *queryparser.Operand) (*queryparser.Operand, error) {
	// Operand cannot be literal, big.Rat or float.
	var c queryparser.Operand
	c.Type = queryparser.TypBigInt
	switch o.Type {
	case queryparser.TypBigInt:
		c.BigInt = o.BigInt
	case queryparser.TypBool:
		return nil, errors.New("cannot convert bool to big.Int")
	case queryparser.TypInt:
		c.BigInt = big.NewInt(o.Int)
	case queryparser.TypUint:
		var b big.Int
		b.SetUint64(o.Uint)
		c.BigInt = &b
	case queryparser.TypObject:
		return nil, errors.New("cannot convert object to big.Int")
	default:
		// TODO(jkline): Log this logic error and all other similar cases.
		return nil, errors.New("cannot convert operand to big.Int")
	}
	return &c, nil
}

func ConvertValueToInt(o *queryparser.Operand) (*queryparser.Operand, error) {
	// Operand cannot be literal, big.Rat or float or uint64.
	var c queryparser.Operand
	c.Type = queryparser.TypInt
	switch o.Type {
	case queryparser.TypBool:
		return nil, errors.New("cannot convert bool to int64")
	case queryparser.TypInt:
		c.Int = o.Int
	case queryparser.TypObject:
		return nil, errors.New("cannot convert object to int64")
	default:
		// TODO(jkline): Log this logic error and all other similar cases.
		return nil, errors.New("cannot convert operand to int64")
	}
	return &c, nil
}

func ConvertValueToUint(o *queryparser.Operand) (*queryparser.Operand, error) {
	// Operand cannot be literal, big.Rat or float or int64.
	var c queryparser.Operand
	c.Type = queryparser.TypUint
	switch o.Type {
	case queryparser.TypBool:
		return nil, errors.New("cannot convert bool to int64")
	case queryparser.TypUint:
		c.Uint = o.Uint
	case queryparser.TypObject:
		return nil, errors.New("cannot convert object to int64")
	default:
		// TODO(jkline): Log this logic error and all other similar cases.
		return nil, errors.New("cannot convert operand to int64")
	}
	return &c, nil
}
