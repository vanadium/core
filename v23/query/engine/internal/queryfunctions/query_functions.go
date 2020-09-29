// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package queryfunctions

import (
	"strings"

	ds "v.io/v23/query/engine/datasource"
	"v.io/v23/query/engine/internal/conversions"
	"v.io/v23/query/engine/internal/queryparser"
	"v.io/v23/query/syncql"
	"v.io/v23/vom"
)

type queryFunc func(ds.Database, int64, []*queryparser.Operand) (*queryparser.Operand, error)
type checkArgsFunc func(ds.Database, int64, []*queryparser.Operand) error

type function struct {
	argTypes      []queryparser.OperandType // TypNil allows any.
	hasVarArgs    bool
	varArgsType   queryparser.OperandType // ignored if !hasVarArgs, TypNil allows any.
	returnType    queryparser.OperandType
	funcAddr      queryFunc
	checkArgsAddr checkArgsFunc
}

var functions map[string]function
var lowercaseFunctions map[string]string // map of lowercase(funcName)->funcName

func init() {
	functions = make(map[string]function)

	// Time Functions
	functions["Time"] = function{[]queryparser.OperandType{queryparser.TypStr, queryparser.TypStr}, false, queryparser.TypNil, queryparser.TypTime, timeFunc, nil}
	functions["Now"] = function{[]queryparser.OperandType{}, false, queryparser.TypNil, queryparser.TypTime, now, nil}
	functions["Year"] = function{[]queryparser.OperandType{queryparser.TypTime, queryparser.TypStr}, false, queryparser.TypNil, queryparser.TypInt, year, secondArgLocationCheck}
	functions["Month"] = function{[]queryparser.OperandType{queryparser.TypTime, queryparser.TypStr}, false, queryparser.TypNil, queryparser.TypInt, month, secondArgLocationCheck}
	functions["Day"] = function{[]queryparser.OperandType{queryparser.TypTime, queryparser.TypStr}, false, queryparser.TypNil, queryparser.TypInt, day, secondArgLocationCheck}
	functions["Hour"] = function{[]queryparser.OperandType{queryparser.TypTime, queryparser.TypStr}, false, queryparser.TypNil, queryparser.TypInt, hour, secondArgLocationCheck}
	functions["Minute"] = function{[]queryparser.OperandType{queryparser.TypTime, queryparser.TypStr}, false, queryparser.TypNil, queryparser.TypInt, minute, secondArgLocationCheck}
	functions["Second"] = function{[]queryparser.OperandType{queryparser.TypTime, queryparser.TypStr}, false, queryparser.TypNil, queryparser.TypInt, second, secondArgLocationCheck}
	functions["Nanosecond"] = function{[]queryparser.OperandType{queryparser.TypTime, queryparser.TypStr}, false, queryparser.TypNil, queryparser.TypInt, nanosecond, secondArgLocationCheck}
	functions["Weekday"] = function{[]queryparser.OperandType{queryparser.TypTime, queryparser.TypStr}, false, queryparser.TypNil, queryparser.TypInt, weekday, secondArgLocationCheck}
	functions["YearDay"] = function{[]queryparser.OperandType{queryparser.TypTime, queryparser.TypStr}, false, queryparser.TypNil, queryparser.TypInt, yearDay, secondArgLocationCheck}

	// String Functions
	functions["Atoi"] = function{[]queryparser.OperandType{queryparser.TypStr}, false, queryparser.TypNil, queryparser.TypInt, atoi, nil}
	functions["Atof"] = function{[]queryparser.OperandType{queryparser.TypStr}, false, queryparser.TypNil, queryparser.TypFloat, atof, nil}
	functions["HtmlEscape"] = function{[]queryparser.OperandType{queryparser.TypStr}, false, queryparser.TypNil, queryparser.TypStr, htmlEscapeFunc, nil}
	functions["HtmlUnescape"] = function{[]queryparser.OperandType{queryparser.TypStr}, false, queryparser.TypNil, queryparser.TypStr, htmlUnescapeFunc, nil}
	functions["Lowercase"] = function{[]queryparser.OperandType{queryparser.TypStr}, false, queryparser.TypNil, queryparser.TypStr, lowerCase, nil}
	functions["Split"] = function{[]queryparser.OperandType{queryparser.TypStr, queryparser.TypStr}, false, queryparser.TypNil, queryparser.TypObject, split, nil}
	functions["Type"] = function{[]queryparser.OperandType{queryparser.TypObject}, false, queryparser.TypNil, queryparser.TypStr, typeFunc, typeFuncFieldCheck}
	functions["Uppercase"] = function{[]queryparser.OperandType{queryparser.TypStr}, false, queryparser.TypNil, queryparser.TypStr, upperCase, nil}
	functions["RuneCount"] = function{[]queryparser.OperandType{queryparser.TypStr}, false, queryparser.TypNil, queryparser.TypInt, runeCount, nil}
	functions["Sprintf"] = function{[]queryparser.OperandType{queryparser.TypStr}, true, queryparser.TypNil, queryparser.TypStr, sprintf, nil}
	functions["Str"] = function{[]queryparser.OperandType{queryparser.TypNil}, false, queryparser.TypNil, queryparser.TypStr, str, nil}
	functions["StrCat"] = function{[]queryparser.OperandType{queryparser.TypStr, queryparser.TypStr}, true, queryparser.TypStr, queryparser.TypStr, strCat, nil}
	functions["StrIndex"] = function{[]queryparser.OperandType{queryparser.TypStr, queryparser.TypStr}, false, queryparser.TypNil, queryparser.TypInt, strIndex, nil}
	functions["StrRepeat"] = function{[]queryparser.OperandType{queryparser.TypStr, queryparser.TypInt}, false, queryparser.TypNil, queryparser.TypStr, strRepeat, nil}
	functions["StrReplace"] = function{[]queryparser.OperandType{queryparser.TypStr, queryparser.TypStr, queryparser.TypStr}, false, queryparser.TypNil, queryparser.TypStr, strReplace, nil}
	functions["StrLastIndex"] = function{[]queryparser.OperandType{queryparser.TypStr, queryparser.TypStr}, false, queryparser.TypNil, queryparser.TypInt, strLastIndex, nil}
	functions["Trim"] = function{[]queryparser.OperandType{queryparser.TypStr}, false, queryparser.TypNil, queryparser.TypStr, trim, nil}
	functions["TrimLeft"] = function{[]queryparser.OperandType{queryparser.TypStr}, false, queryparser.TypNil, queryparser.TypStr, trimLeft, nil}
	functions["TrimRight"] = function{[]queryparser.OperandType{queryparser.TypStr}, false, queryparser.TypNil, queryparser.TypStr, trimRight, nil}

	// Math functions
	functions["Ceiling"] = function{[]queryparser.OperandType{queryparser.TypFloat}, false, queryparser.TypNil, queryparser.TypFloat, ceilingFunc, nil}
	functions["Floor"] = function{[]queryparser.OperandType{queryparser.TypFloat}, false, queryparser.TypNil, queryparser.TypFloat, floorFunc, nil}
	functions["Inf"] = function{[]queryparser.OperandType{queryparser.TypInt}, false, queryparser.TypNil, queryparser.TypFloat, infFunc, nil}
	functions["IsInf"] = function{[]queryparser.OperandType{queryparser.TypFloat, queryparser.TypInt}, false, queryparser.TypNil, queryparser.TypBool, isInfFunc, nil}
	functions["IsNaN"] = function{[]queryparser.OperandType{queryparser.TypFloat}, false, queryparser.TypNil, queryparser.TypBool, isNanFunc, nil}
	functions["Log"] = function{[]queryparser.OperandType{queryparser.TypFloat}, false, queryparser.TypNil, queryparser.TypFloat, logFunc, nil}
	functions["Log10"] = function{[]queryparser.OperandType{queryparser.TypFloat}, false, queryparser.TypNil, queryparser.TypFloat, log10Func, nil}
	functions["NaN"] = function{[]queryparser.OperandType{}, false, queryparser.TypNil, queryparser.TypFloat, nanFunc, nil}
	functions["Pow"] = function{[]queryparser.OperandType{queryparser.TypFloat, queryparser.TypFloat}, false, queryparser.TypNil, queryparser.TypFloat, powFunc, nil}
	functions["Pow10"] = function{[]queryparser.OperandType{queryparser.TypInt}, false, queryparser.TypNil, queryparser.TypFloat, pow10Func, nil}
	functions["Mod"] = function{[]queryparser.OperandType{queryparser.TypFloat, queryparser.TypFloat}, false, queryparser.TypNil, queryparser.TypFloat, modFunc, nil}
	functions["Truncate"] = function{[]queryparser.OperandType{queryparser.TypFloat}, false, queryparser.TypNil, queryparser.TypFloat, truncateFunc, nil}
	functions["Remainder"] = function{[]queryparser.OperandType{queryparser.TypFloat, queryparser.TypFloat}, false, queryparser.TypNil, queryparser.TypFloat, remainderFunc, nil}

	// TODO(jkline): Make len work with more types.
	functions["Len"] = function{[]queryparser.OperandType{queryparser.TypObject}, false, queryparser.TypNil, queryparser.TypInt, lenFunc, nil}

	// Build lowercaseFuncName->funcName
	lowercaseFunctions = make(map[string]string)
	for f := range functions {
		lowercaseFunctions[strings.ToLower(f)] = f
	}
}

// Check that function exists and that the number of args passed matches the spec.
// Call query_functions.CheckFunction.  This will check for, to the extent possible, correct types.
// Furthermore, it may execute the function if the function takes no args or
// takes only literal args (or an arg that is a function that is also executed
// early).  CheckFunction will fill in arg types, return types and may fill in
// Computed and RetValue.
func CheckFunction(db ds.Database, f *queryparser.Function) error {
	entry, err := lookupFuncName(db, f)
	if err != nil {
		return err
	}
	f.ArgTypes = entry.argTypes
	f.RetType = entry.returnType
	if !entry.hasVarArgs && len(f.Args) != len(entry.argTypes) {
		return syncql.ErrorfFunctionArgCount(db.GetContext(), "[%v]function '%v' expects %vargs, found: %v", f.Off, f.Name, int64(len(f.ArgTypes)), int64(len(f.Args)))
	}
	if entry.hasVarArgs && len(f.Args) < len(entry.argTypes) {
		return syncql.ErrorfFunctionAtLeastArgCount(db.GetContext(), "[%v]function '%v' expects at least %vargs, found: %v", f.Off, f.Name, int64(len(f.ArgTypes)), int64(len(f.Args)))
	}
	// Standard check for types of fixed and var args
	if err = argsStandardCheck(db, f.Off, entry, f.Args); err != nil {
		return err
	}
	// Check if the function can be executed now.
	// If any arg is not a literal and not a function that has been already executed,
	// then okToExecuteNow will be set to false.

	executeNow := func() bool {
		for _, arg := range f.Args {
			switch arg.Type {
			case queryparser.TypBigInt, queryparser.TypBigRat, queryparser.TypBool, queryparser.TypFloat, queryparser.TypInt, queryparser.TypStr, queryparser.TypTime, queryparser.TypUint:
				// do nothing
			case queryparser.TypFunction:
				if !arg.Function.Computed {
					return false
				}
			default:
				return false
			}
		}
		return true
	}

	okToExecuteNow := executeNow()
	// If all of the functions args are literals or already computed functions,
	// execute this function now and save the result.
	if okToExecuteNow {
		op, err := ExecFunction(db, f, f.Args)
		if err != nil {
			return err
		}
		f.Computed = true
		f.RetValue = op
		return nil
	}
	// We can't execute now, but give the function a chance to complain
	// about the arguments that it can check now.
	return FuncCheck(db, f, f.Args)
}

func lookupFuncName(db ds.Database, f *queryparser.Function) (*function, error) {
	entry, ok := functions[f.Name]
	if !ok {
		// No such function, is the case wrong?
		correctCase, ok := lowercaseFunctions[strings.ToLower(f.Name)]
		if !ok {
			return nil, syncql.ErrorfFunctionNotFound(db.GetContext(), "[%v]function '%v' not found", f.Off, f.Name)
		}
		// the case is wrong
		return nil, syncql.ErrorfDidYouMeanFunction(db.GetContext(), "[%v]did you mean: '%v'?", f.Off, correctCase)
	}
	return &entry, nil
}

func FuncCheck(db ds.Database, f *queryparser.Function, args []*queryparser.Operand) error {
	if entry, err := lookupFuncName(db, f); err != nil {
		return err
	} else if entry.checkArgsAddr != nil {
		if err := entry.checkArgsAddr(db, f.Off, args); err != nil {
			return err
		}
	}
	return nil
}

func ExecFunction(db ds.Database, f *queryparser.Function, args []*queryparser.Operand) (*queryparser.Operand, error) {
	entry, err := lookupFuncName(db, f)
	if err != nil {
		return nil, err
	}
	retValue, err := entry.funcAddr(db, f.Off, args)
	if err != nil {
		return nil, err
	}
	return retValue, nil
}

func ConvertFunctionRetValueToRawBytes(o *queryparser.Operand) *vom.RawBytes {
	if o == nil {
		return vom.RawBytesOf(nil)
	}
	switch o.Type {
	case queryparser.TypBool:
		return vom.RawBytesOf(o.Bool)
	case queryparser.TypFloat:
		return vom.RawBytesOf(o.Float)
	case queryparser.TypInt:
		return vom.RawBytesOf(o.Int)
	case queryparser.TypStr:
		return vom.RawBytesOf(o.Str)
	case queryparser.TypTime:
		return vom.RawBytesOf(o.Time)
	case queryparser.TypObject:
		return vom.RawBytesOf(o.Object)
	case queryparser.TypUint:
		return vom.RawBytesOf(o.Uint)
	default:
		// Other types can't be converted and *shouldn't* be returned
		// from a function.  This case will result in a nil for this
		// column in the row.
		return vom.RawBytesOf(nil)
	}
}

func makeStrOp(off int64, s string) *queryparser.Operand {
	var o queryparser.Operand
	o.Off = off
	o.Type = queryparser.TypStr
	o.Str = s
	return &o
}

func makeBoolOp(off int64, b bool) *queryparser.Operand {
	var o queryparser.Operand
	o.Off = off
	o.Type = queryparser.TypBool
	o.Bool = b
	return &o
}

func makeIntOp(off int64, i int64) *queryparser.Operand {
	var o queryparser.Operand
	o.Off = off
	o.Type = queryparser.TypInt
	o.Int = i
	return &o
}

func makeFloatOp(off int64, r float64) *queryparser.Operand {
	var o queryparser.Operand
	o.Off = off
	o.Type = queryparser.TypFloat
	o.Float = r
	return &o
}

func checkArg(db ds.Database, off int64, argType queryparser.OperandType, arg *queryparser.Operand) error { //nolint:gocyclo
	// We can't check unless the arg is a literal or an already computed function,
	var operandToConvert *queryparser.Operand
	switch arg.Type {
	case queryparser.TypBigInt, queryparser.TypBigRat, queryparser.TypBool, queryparser.TypFloat, queryparser.TypInt, queryparser.TypStr, queryparser.TypTime, queryparser.TypUint:
		operandToConvert = arg
	case queryparser.TypFunction:
		if arg.Function.Computed {
			operandToConvert = arg.Function.RetValue
		} else {
			return nil // arg is not yet resolved, we can't check
		}
	default:
		return nil // arg is not yet resolved, we can't check
	}
	// make sure it can be converted to argType.
	var err error
	switch argType {
	case queryparser.TypBigInt:
		_, err = conversions.ConvertValueToBigInt(operandToConvert)
		if err != nil {
			err = syncql.ErrorfBigIntConversionError(db.GetContext(), "[%v]Can't convert to BigInt: %v", arg.Off, err)
		}
	case queryparser.TypBigRat:
		_, err = conversions.ConvertValueToBigRat(operandToConvert)
		if err != nil {
			err = syncql.ErrorfBigRatConversionError(db.GetContext(), "[%v]can't convert to BigRat: %v", arg.Off, err)
		}
	case queryparser.TypBool:
		_, err = conversions.ConvertValueToBool(operandToConvert)
		if err != nil {
			err = syncql.ErrorfBoolConversionError(db.GetContext(), "[%v]can't convert to Bool: %v", arg.Off, err)
		}
	case queryparser.TypFloat:
		_, err = conversions.ConvertValueToFloat(operandToConvert)
		if err != nil {
			err = syncql.ErrorfFloatConversionError(db.GetContext(), "[%v]can't convert to float: %v", arg.Off, err)
		}
	case queryparser.TypInt:
		_, err = conversions.ConvertValueToInt(operandToConvert)
		if err != nil {
			err = syncql.ErrorfIntConversionError(db.GetContext(), "[%v]can't convert to int: %v", arg.Off, err)
		}
	case queryparser.TypStr:
		_, err = conversions.ConvertValueToString(operandToConvert)
		if err != nil {
			err = syncql.ErrorfStringConversionError(db.GetContext(), "[%v]can't convert to string: %v", arg.Off, err)
		}
	case queryparser.TypTime:
		_, err = conversions.ConvertValueToTime(operandToConvert)
		if err != nil {
			err = syncql.ErrorfTimeConversionError(db.GetContext(), "[%v]can't convert to time: %v", arg.Off, err)
		}
	case queryparser.TypUint:
		_, err = conversions.ConvertValueToUint(operandToConvert)
		if err != nil {
			err = syncql.ErrorfUintConversionError(db.GetContext(), "[%v]can't convert to Uint: %v", arg.Off, err)
		}
	}
	return err
}

// Check types of fixed args.  For functions that take varargs, check that the type of
// any varargs matches the type specified.
func argsStandardCheck(db ds.Database, off int64, f *function, args []*queryparser.Operand) error {
	// Check types of required args.
	for i := 0; i < len(f.argTypes); i++ {
		if err := checkArg(db, off, f.argTypes[i], args[i]); err != nil {
			return err
		}
	}
	// Check types of varargs.
	if f.hasVarArgs {
		for i := len(f.argTypes); i < len(args); i++ {
			if f.varArgsType != queryparser.TypNil {
				if err := checkArg(db, off, f.varArgsType, args[i]); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
