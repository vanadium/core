// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"fmt"
	"reflect"

	ds "v.io/v23/query/engine/datasource"
	"v.io/v23/query/engine/internal/conversions"
	"v.io/v23/query/engine/internal/querychecker"
	"v.io/v23/query/engine/internal/queryfunctions"
	"v.io/v23/query/engine/internal/queryparser"
	"v.io/v23/query/syncql"
	"v.io/v23/vdl"
)

func Eval(db ds.Database, k string, v *vdl.Value, e *queryparser.Expression) bool {
	if querychecker.IsLogicalOperator(e.Operator) {
		return evalLogicalOperators(db, k, v, e)
	}
	return evalComparisonOperators(db, k, v, e)
}

func evalLogicalOperators(db ds.Database, k string, v *vdl.Value, e *queryparser.Expression) bool {
	switch e.Operator.Type {
	case queryparser.And:
		return Eval(db, k, v, e.Operand1.Expr) && Eval(db, k, v, e.Operand2.Expr)
	case queryparser.Or:
		return Eval(db, k, v, e.Operand1.Expr) || Eval(db, k, v, e.Operand2.Expr)
	default:
		// TODO(jkline): Log this logic error and all other similar cases.
		return false
	}
}

func evalComparisonOperators(db ds.Database, k string, v *vdl.Value, e *queryparser.Expression) bool { //nolint:gocyclo
	lhsValue := resolveOperand(db, k, v, e.Operand1)
	// Check for an is nil expression (i.e., v[.<field>...] is nil).
	// These expressions evaluate to true if the field cannot be resolved.
	if e.Operator.Type == queryparser.Is && e.Operand2.Type == queryparser.TypNil {
		return lhsValue == nil
	}
	if e.Operator.Type == queryparser.IsNot && e.Operand2.Type == queryparser.TypNil {
		return lhsValue != nil
	}
	// For anything but "is[not] nil" (which is handled above), an unresolved operator
	// results in the expression evaluating to false.
	if lhsValue == nil {
		return false
	}
	rhsValue := resolveOperand(db, k, v, e.Operand2)
	if rhsValue == nil {
		return false
	}
	// coerce operands so they are comparable
	var err error
	lhsValue, rhsValue, err = coerceValues(lhsValue, rhsValue)
	if err != nil {
		return false // If operands can't be coerced to compare, expr evals to false.
	}
	// Do the compare
	switch lhsValue.Type {
	case queryparser.TypBigInt:
		return compareBigInts(lhsValue, rhsValue, e.Operator)
	case queryparser.TypBigRat:
		return compareBigRats(lhsValue, rhsValue, e.Operator)
	case queryparser.TypBool:
		return compareBools(lhsValue, rhsValue, e.Operator)
	case queryparser.TypFloat:
		return compareFloats(lhsValue, rhsValue, e.Operator)
	case queryparser.TypInt:
		return compareInts(lhsValue, rhsValue, e.Operator)
	case queryparser.TypStr:
		return compareStrings(lhsValue, rhsValue, e.Operator)
	case queryparser.TypUint:
		return compareUints(lhsValue, rhsValue, e.Operator)
	case queryparser.TypTime:
		return compareTimes(lhsValue, rhsValue, e.Operator)
	case queryparser.TypObject:
		return compareObjects(lhsValue, rhsValue, e.Operator)
	}
	return false
}

func coerceValues(lhsValue, rhsValue *queryparser.Operand) (*queryparser.Operand, *queryparser.Operand, error) { //nolint:gocyclo
	// TODO(jkline): explore using vdl for coercions ( https://vanadium.github.io/designdocs/vdl-spec.html#conversions ).
	var err error
	// If either operand is a string, convert the other to a string.
	if lhsValue.Type == queryparser.TypStr || rhsValue.Type == queryparser.TypStr {
		if lhsValue, err = conversions.ConvertValueToString(lhsValue); err != nil {
			return nil, nil, err
		}
		if rhsValue, err = conversions.ConvertValueToString(rhsValue); err != nil {
			return nil, nil, err
		}
		return lhsValue, rhsValue, nil
	}
	// If either operand is a big rat, convert both to a big rat.
	// Also, if one operand is a float and the other is a big int,
	// convert both to big rats.
	if lhsValue.Type == queryparser.TypBigRat || rhsValue.Type == queryparser.TypBigRat || (lhsValue.Type == queryparser.TypBigInt && rhsValue.Type == queryparser.TypFloat) || (lhsValue.Type == queryparser.TypFloat && rhsValue.Type == queryparser.TypBigInt) {
		if lhsValue, err = conversions.ConvertValueToBigRat(lhsValue); err != nil {
			return nil, nil, err
		}
		if rhsValue, err = conversions.ConvertValueToBigRat(rhsValue); err != nil {
			return nil, nil, err
		}
		return lhsValue, rhsValue, nil
	}
	// If either operand is a float, convert the other to a float.
	if lhsValue.Type == queryparser.TypFloat || rhsValue.Type == queryparser.TypFloat {
		if lhsValue, err = conversions.ConvertValueToFloat(lhsValue); err != nil {
			return nil, nil, err
		}
		if rhsValue, err = conversions.ConvertValueToFloat(rhsValue); err != nil {
			return nil, nil, err
		}
		return lhsValue, rhsValue, nil
	}
	// If either operand is a big int, convert both to a big int.
	// Also, if one operand is a uint64 and the other is an int64, convert both to big ints.
	if lhsValue.Type == queryparser.TypBigInt || rhsValue.Type == queryparser.TypBigInt || (lhsValue.Type == queryparser.TypUint && rhsValue.Type == queryparser.TypInt) || (lhsValue.Type == queryparser.TypInt && rhsValue.Type == queryparser.TypUint) {
		if lhsValue, err = conversions.ConvertValueToBigInt(lhsValue); err != nil {
			return nil, nil, err
		}
		if rhsValue, err = conversions.ConvertValueToBigInt(rhsValue); err != nil {
			return nil, nil, err
		}
		return lhsValue, rhsValue, nil
	}
	// If either operand is an int64, convert the other to int64.
	if lhsValue.Type == queryparser.TypInt || rhsValue.Type == queryparser.TypInt {
		if lhsValue, err = conversions.ConvertValueToInt(lhsValue); err != nil {
			return nil, nil, err
		}
		if rhsValue, err = conversions.ConvertValueToInt(rhsValue); err != nil {
			return nil, nil, err
		}
		return lhsValue, rhsValue, nil
	}
	// If either operand is an uint64, convert the other to uint64.
	if lhsValue.Type == queryparser.TypUint || rhsValue.Type == queryparser.TypUint {
		if lhsValue, err = conversions.ConvertValueToUint(lhsValue); err != nil {
			return nil, nil, err
		}
		if rhsValue, err = conversions.ConvertValueToUint(rhsValue); err != nil {
			return nil, nil, err
		}
		return lhsValue, rhsValue, nil
	}
	// Must be the same at this point.
	if lhsValue.Type != rhsValue.Type {
		return nil, nil, fmt.Errorf("Logic error: expected like types, got: %v, %v", lhsValue, rhsValue)
	}

	return lhsValue, rhsValue, nil
}

func compareBools(lhsValue, rhsValue *queryparser.Operand, oper *queryparser.BinaryOperator) bool {
	switch oper.Type {
	case queryparser.Equal:
		return lhsValue.Bool == rhsValue.Bool
	case queryparser.NotEqual:
		return lhsValue.Bool != rhsValue.Bool
	default:
		// TODO(jkline): Log this logic error and all other similar cases.
		return false
	}
}

func compareBigInts(lhsValue, rhsValue *queryparser.Operand, oper *queryparser.BinaryOperator) bool {
	switch oper.Type {
	case queryparser.Equal:
		return lhsValue.BigInt.Cmp(rhsValue.BigInt) == 0
	case queryparser.NotEqual:
		return lhsValue.BigInt.Cmp(rhsValue.BigInt) != 0
	case queryparser.LessThan:
		return lhsValue.BigInt.Cmp(rhsValue.BigInt) < 0
	case queryparser.LessThanOrEqual:
		return lhsValue.BigInt.Cmp(rhsValue.BigInt) <= 0
	case queryparser.GreaterThan:
		return lhsValue.BigInt.Cmp(rhsValue.BigInt) > 0
	case queryparser.GreaterThanOrEqual:
		return lhsValue.BigInt.Cmp(rhsValue.BigInt) >= 0
	default:
		// TODO(jkline): Log this logic error and all other similar cases.
		return false
	}
}

func compareBigRats(lhsValue, rhsValue *queryparser.Operand, oper *queryparser.BinaryOperator) bool {
	switch oper.Type {
	case queryparser.Equal:
		return lhsValue.BigRat.Cmp(rhsValue.BigRat) == 0
	case queryparser.NotEqual:
		return lhsValue.BigRat.Cmp(rhsValue.BigRat) != 0
	case queryparser.LessThan:
		return lhsValue.BigRat.Cmp(rhsValue.BigRat) < 0
	case queryparser.LessThanOrEqual:
		return lhsValue.BigRat.Cmp(rhsValue.BigRat) <= 0
	case queryparser.GreaterThan:
		return lhsValue.BigRat.Cmp(rhsValue.BigRat) > 0
	case queryparser.GreaterThanOrEqual:
		return lhsValue.BigRat.Cmp(rhsValue.BigRat) >= 0
	default:
		// TODO(jkline): Log this logic error and all other similar cases.
		return false
	}
}

func compareFloats(lhsValue, rhsValue *queryparser.Operand, oper *queryparser.BinaryOperator) bool {
	switch oper.Type {
	case queryparser.Equal:
		return lhsValue.Float == rhsValue.Float
	case queryparser.NotEqual:
		return lhsValue.Float != rhsValue.Float
	case queryparser.LessThan:
		return lhsValue.Float < rhsValue.Float
	case queryparser.LessThanOrEqual:
		return lhsValue.Float <= rhsValue.Float
	case queryparser.GreaterThan:
		return lhsValue.Float > rhsValue.Float
	case queryparser.GreaterThanOrEqual:
		return lhsValue.Float >= rhsValue.Float
	default:
		// TODO(jkline): Log this logic error and all other similar cases.
		return false
	}
}

func compareInts(lhsValue, rhsValue *queryparser.Operand, oper *queryparser.BinaryOperator) bool {
	switch oper.Type {
	case queryparser.Equal:
		return lhsValue.Int == rhsValue.Int
	case queryparser.NotEqual:
		return lhsValue.Int != rhsValue.Int
	case queryparser.LessThan:
		return lhsValue.Int < rhsValue.Int
	case queryparser.LessThanOrEqual:
		return lhsValue.Int <= rhsValue.Int
	case queryparser.GreaterThan:
		return lhsValue.Int > rhsValue.Int
	case queryparser.GreaterThanOrEqual:
		return lhsValue.Int >= rhsValue.Int
	default:
		// TODO(jkline): Log this logic error and all other similar cases.
		return false
	}
}

func compareUints(lhsValue, rhsValue *queryparser.Operand, oper *queryparser.BinaryOperator) bool {
	switch oper.Type {
	case queryparser.Equal:
		return lhsValue.Uint == rhsValue.Uint
	case queryparser.NotEqual:
		return lhsValue.Uint != rhsValue.Uint
	case queryparser.LessThan:
		return lhsValue.Uint < rhsValue.Uint
	case queryparser.LessThanOrEqual:
		return lhsValue.Uint <= rhsValue.Uint
	case queryparser.GreaterThan:
		return lhsValue.Uint > rhsValue.Uint
	case queryparser.GreaterThanOrEqual:
		return lhsValue.Uint >= rhsValue.Uint
	default:
		// TODO(jkline): Log this logic error and all other similar cases.
		return false
	}
}

func compareStrings(lhsValue, rhsValue *queryparser.Operand, oper *queryparser.BinaryOperator) bool {
	switch oper.Type {
	case queryparser.Equal:
		return lhsValue.Str == rhsValue.Str
	case queryparser.NotEqual:
		return lhsValue.Str != rhsValue.Str
	case queryparser.LessThan:
		return lhsValue.Str < rhsValue.Str
	case queryparser.LessThanOrEqual:
		return lhsValue.Str <= rhsValue.Str
	case queryparser.GreaterThan:
		return lhsValue.Str > rhsValue.Str
	case queryparser.GreaterThanOrEqual:
		return lhsValue.Str >= rhsValue.Str
	case queryparser.Like:
		return rhsValue.Pattern.MatchString(lhsValue.Str)
	case queryparser.NotLike:
		return !rhsValue.Pattern.MatchString(lhsValue.Str)
	default:
		// TODO(jkline): Log this logic error and all other similar cases.
		return false
	}
}

func compareTimes(lhsValue, rhsValue *queryparser.Operand, oper *queryparser.BinaryOperator) bool {
	switch oper.Type {
	case queryparser.Equal:
		return lhsValue.Time.Equal(rhsValue.Time)
	case queryparser.NotEqual:
		return !lhsValue.Time.Equal(rhsValue.Time)
	case queryparser.LessThan:
		return lhsValue.Time.Before(rhsValue.Time)
	case queryparser.LessThanOrEqual:
		return lhsValue.Time.Before(rhsValue.Time) || lhsValue.Time.Equal(rhsValue.Time)
	case queryparser.GreaterThan:
		return lhsValue.Time.After(rhsValue.Time)
	case queryparser.GreaterThanOrEqual:
		return lhsValue.Time.After(rhsValue.Time) || lhsValue.Time.Equal(rhsValue.Time)
	default:
		// TODO(jkline): Log this logic error and all other similar cases.
		return false
	}
}

func compareObjects(lhsValue, rhsValue *queryparser.Operand, oper *queryparser.BinaryOperator) bool {
	switch oper.Type {
	case queryparser.Equal:
		return reflect.DeepEqual(lhsValue.Object, rhsValue.Object)
	case queryparser.NotEqual:
		return !reflect.DeepEqual(lhsValue.Object, rhsValue.Object)
	default: // other operands are non-sensical
		return false
	}
}

func resolveArgsAndExecFunction(db ds.Database, k string, v *vdl.Value, f *queryparser.Function) (*queryparser.Operand, error) {
	// Resolve the function's arguments
	callingArgs := []*queryparser.Operand{}
	for _, arg := range f.Args {
		resolvedArg := resolveOperand(db, k, v, arg)
		if resolvedArg == nil {
			return nil, syncql.ErrorfFunctionArgBad(db.GetContext(), "[%v]Function '%v' arg '%v' could not be resolved.", arg.Off, f.Name, arg.String())
		}
		callingArgs = append(callingArgs, resolvedArg)
	}
	// Exec the function
	retValue, err := queryfunctions.ExecFunction(db, f, callingArgs)
	if err != nil {
		return nil, err
	}
	return retValue, nil
}

func resolveOperand(db ds.Database, k string, v *vdl.Value, o *queryparser.Operand) *queryparser.Operand {
	if o.Type == queryparser.TypFunction {
		// Note: if the function was computed at check time, the operand is replaced
		// in the parse tree with the return value.  As such, thre is no need to check
		// the computed field.
		if retValue, err := resolveArgsAndExecFunction(db, k, v, o.Function); err == nil {
			return retValue
		}
		// Per spec, function errors resolve to nil
		return nil
	}
	if o.Type != queryparser.TypField {
		return o
	}
	value := ResolveField(db, k, v, o.Column)
	if value.IsNil() {
		return nil
	}
	newOp, err := queryparser.ConvertValueToAnOperand(value, o.Off)
	if err != nil {
		return nil
	}
	return newOp
}

// Auto-dereference Any and Optional values
func autoDereference(o *vdl.Value) *vdl.Value {
	for o.Kind() == vdl.Any || o.Kind() == vdl.Optional {
		o = o.Elem()
		if o == nil {
			break
		}
	}
	if o == nil {
		o = vdl.ValueOf(nil)
	}
	return o
}

// Resolve object with the key(s) (if a key(s) was specified).  That is, resolve the object with the
// value of what is specified in brackets (for maps, sets. arrays and lists).
// If no key was specified, just return the object.
// If a key was specified, but the object is not map, set, array or list: return nil.
// If the key resolved to nil, return nil.
// If the key can't be converted to the required type, return nil.
// For arrays/lists, if the index is out of bounds, return nil.
// For maps, if key not found, return nil.
// For sets, if key found, return true, else return false.
func resolveWithKey(db ds.Database, k string, v *vdl.Value, object *vdl.Value, segment queryparser.Segment) *vdl.Value {
	for _, key := range segment.Keys {
		o := resolveOperand(db, k, v, key)
		if o == nil {
			return vdl.ValueOf(nil)
		}
		proposedKey := valueFromResolvedOperand(o)
		if proposedKey == nil {
			return vdl.ValueOf(nil)
		}
		switch object.Kind() {
		case vdl.Array, vdl.List:
			// convert key to int
			// vdl's Index function wants an int.
			// vdl can't make an int.
			// int is guaranteed to be at least 32-bits.
			// So have vdl make an int32 and then convert it to an int.
			index32 := vdl.IntValue(vdl.Int32Type, 0)
			if err := vdl.Convert(index32, proposedKey); err != nil {
				return vdl.ValueOf(nil)
			}
			index := int(index32.Int())
			if index < 0 || index >= object.Len() {
				return vdl.ValueOf(nil)
			}
			object = object.Index(index)
		case vdl.Map, vdl.Set:
			reqKeyType := object.Type().Key()
			keyVal := vdl.ZeroValue(reqKeyType)
			if err := vdl.Convert(keyVal, proposedKey); err != nil {
				return vdl.ValueOf(nil)
			}
			if object.Kind() == vdl.Map {
				rv := object.MapIndex(keyVal)
				if rv != nil {
					object = rv
				} else {
					return vdl.ValueOf(nil)
				}
			} else { // vdl.Set
				object = vdl.ValueOf(object.ContainsKey(keyVal))
			}
		default:
			return vdl.ValueOf(nil)
		}
	}
	return object
}

// Return the value of a non-nil *Operand that has been resolved by resolveOperand.
func valueFromResolvedOperand(o *queryparser.Operand) interface{} {
	// This switch contains the possible types returned from resolveOperand.
	switch o.Type {
	case queryparser.TypBigInt:
		return o.BigInt
	case queryparser.TypBigRat:
		return o.BigRat
	case queryparser.TypBool:
		return o.Bool
	case queryparser.TypFloat:
		return o.Float
	case queryparser.TypInt:
		return o.Int
	case queryparser.TypNil:
		return nil
	case queryparser.TypStr:
		return o.Str
	case queryparser.TypTime:
		return o.Time
	case queryparser.TypObject:
		return o.Object
	case queryparser.TypUint:
		return o.Uint
	}
	return nil
}

// Resolve a field.
func ResolveField(db ds.Database, k string, v *vdl.Value, f *queryparser.Field) *vdl.Value {
	if querychecker.IsKeyField(f) {
		return vdl.StringValue(nil, k)
	}
	// Auto-dereference Any and Optional values
	v = autoDereference(v)

	object := v
	segments := f.Segments
	// Does v contain a key?
	object = resolveWithKey(db, k, v, object, segments[0])

	// More segments?
	for i := 1; i < len(segments); i++ {
		// Auto-dereference Any and Optional values
		object = autoDereference(object)
		// object must be a struct in order to look for the next segment.
		switch {
		case object.Kind() == vdl.Struct:
			if object = object.StructFieldByName(segments[i].Value); object == nil {
				return vdl.ValueOf(nil)
			}
			object = resolveWithKey(db, k, v, object, segments[i])
		case object.Kind() == vdl.Union:
			unionType := object.Type()
			idx, tempValue := object.UnionField()
			if segments[i].Value == unionType.Field(idx).Name {
				object = tempValue
			} else {
				return vdl.ValueOf(nil)
			}
			object = resolveWithKey(db, k, v, object, segments[i])
		default:
			return vdl.ValueOf(nil)
		}
	}
	return object
}

// EvalWhereUsingOnlyKey return type.  See that function for details.
type EvalWithKeyResult int

const (
	Include EvalWithKeyResult = iota
	Exclude
	FetchValue
)

// Evaluate the where clause to determine if the row should be selected, but do so using only
// the key.  Possible returns are:
// Include: the row should Included in the results
// Exclude: the row should NOT be Included
// FetchValue: the value and/or type of the value are required to determine if row should be Included.
// The above decision is accomplished by evaluating all expressions which compare the key
// with a string literal and substituing false for all other expressions.  If the result is true,
// Include is returned.
// If the result is false, but no other expressions were encountered, Exclude is returned; else,
// FetchValue is returned indicating the value must be fetched in order to determine if the row
// should be Included in the results.
func EvalWhereUsingOnlyKey(db ds.Database, w *queryparser.WhereClause, k string) EvalWithKeyResult {
	if w == nil { // all rows will be in result
		return Include
	}
	return evalExprUsingOnlyKey(db, w.Expr, k)
}

func evalExprUsingOnlyKey(db ds.Database, e *queryparser.Expression, k string) EvalWithKeyResult { //nolint:gocyclo
	switch e.Operator.Type {
	case queryparser.And:
		op1Result := evalExprUsingOnlyKey(db, e.Operand1.Expr, k)
		op2Result := evalExprUsingOnlyKey(db, e.Operand2.Expr, k)
		switch {
		case op1Result == Include && op2Result == Include:
			return Include
		case op1Result == Exclude || op2Result == Exclude:
			// One of the operands evaluated to Exclude.
			// As such, the value is not needed to reject the row.
			return Exclude
		default:
			return FetchValue
		}
	case queryparser.Or:
		op1Result := evalExprUsingOnlyKey(db, e.Operand1.Expr, k)
		op2Result := evalExprUsingOnlyKey(db, e.Operand2.Expr, k)
		switch {
		case op1Result == Include || op2Result == Include:
			return Include
		case op1Result == Exclude && op2Result == Exclude:
			return Exclude
		default:
			return FetchValue
		}
	default:
		if !querychecker.ContainsKeyOperand(e) {
			return FetchValue
		}
		// at least one operand is a key
		// May still need to fetch the value (if
		// one of the operands is a value field or a function).
		switch {
		case querychecker.ContainsFunctionOperand(e) || querychecker.ContainsValueFieldOperand(e):
			return FetchValue
		case evalComparisonOperators(db, k, nil, e):
			return Include
		default:
			return Exclude
		}
	}
}
