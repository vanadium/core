// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package querychecker

import (
	"sort"

	ds "v.io/v23/query/engine/datasource"
	"v.io/v23/query/engine/internal/queryfunctions"
	"v.io/v23/query/engine/internal/queryparser"
	"v.io/v23/query/pattern"
	"v.io/v23/query/syncql"
	"v.io/v23/vdl"
)

const (
	MaxRangeLimit    = ""
	lowercaseKFormat = "[%v]did you mean: 'k'?"
	lowercaseVFormat = "[%v]did you mean: 'v'?"
)

var (
	StringFieldRangeAll = ds.StringFieldRange{Start: "", Limit: MaxRangeLimit}
)

func Check(db ds.Database, s *queryparser.Statement) error {
	switch sel := (*s).(type) {
	case queryparser.SelectStatement:
		return checkSelectStatement(db, &sel)
	case queryparser.DeleteStatement:
		return checkDeleteStatement(db, &sel)
	default:
		return syncql.ErrorfCheckOfUnknownStatementType(db.GetContext(), "[%v]cannot semantically check unknown statement type.", (*s).Offset())
	}
}

func checkSelectStatement(db ds.Database, s *queryparser.SelectStatement) error {
	if err := checkSelectClause(db, s.Select); err != nil {
		return err
	}
	if err := checkFromClause(db, s.From, false); err != nil {
		return err
	}
	if err := checkEscapeClause(db, s.Escape); err != nil {
		return err
	}
	if err := checkWhereClause(db, s.Where, s.Escape); err != nil {
		return err
	}
	if err := checkLimitClause(db, s.Limit); err != nil {
		return err
	}
	if err := checkResultsOffsetClause(db, s.ResultsOffset); err != nil {
		return err
	}
	return nil
}

func checkDeleteStatement(db ds.Database, s *queryparser.DeleteStatement) error {
	if err := checkFromClause(db, s.From, true); err != nil {
		return err
	}
	if err := checkEscapeClause(db, s.Escape); err != nil {
		return err
	}
	if err := checkWhereClause(db, s.Where, s.Escape); err != nil {
		return err
	}
	if err := checkLimitClause(db, s.Limit); err != nil {
		return err
	}
	return nil
}

// Check select clause.  Fields can be 'k' and v[{.<ident>}...]
func checkSelectClause(db ds.Database, s *queryparser.SelectClause) error {
	for _, selector := range s.Selectors {
		switch selector.Type {
		case queryparser.TypSelField:
			switch selector.Field.Segments[0].Value {
			case "k":
				if len(selector.Field.Segments) > 1 {
					return syncql.ErrorfDotNotationDisallowedForKey(db.GetContext(), "[%v]dot notation may not be used on a key field.", selector.Field.Segments[1].Off)
				}
			case "v":
				// Nothing to check.
			case "K":
				// Be nice and warn of mistakenly capped 'K'.
				return syncql.ErrorfDidYouMeanLowercaseK(db.GetContext(), lowercaseKFormat, selector.Field.Segments[0].Off)
			case "V":
				// Be nice and warn of mistakenly capped 'V'.
				return syncql.ErrorfDidYouMeanLowercaseV(db.GetContext(), lowercaseVFormat, selector.Field.Segments[0].Off)
			default:
				return syncql.ErrorfInvalidSelectField(db.GetContext(), "[%v]select field must be 'k' or 'v[{.<ident>}...]'", selector.Field.Segments[0].Off)
			}
		case queryparser.TypSelFunc:
			err := queryfunctions.CheckFunction(db, selector.Function)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// Check from clause.  Table must exist in the database.
func checkFromClause(db ds.Database, f *queryparser.FromClause, writeAccessReq bool) error {
	var err error
	f.Table.DBTable, err = db.GetTable(f.Table.Name, writeAccessReq)
	if err != nil {
		return syncql.ErrorfTableCantAccess(db.GetContext(), "[%v]table %v does not exist (or cannot be accessed): %v", f.Table.Off, f.Table.Name, err)
	}
	return nil
}

// Check where clause.
func checkWhereClause(db ds.Database, w *queryparser.WhereClause, ec *queryparser.EscapeClause) error {
	if w == nil {
		return nil
	}
	return checkExpression(db, w.Expr, ec)
}

func checkExpression(db ds.Database, e *queryparser.Expression, ec *queryparser.EscapeClause) error { //nolint:gocyclo
	if err := checkOperand(db, e.Operand1, ec); err != nil {
		return err
	}
	if err := checkOperand(db, e.Operand2, ec); err != nil {
		return err
	}

	// Like expressions require operand2 to be a string literal that must be validated.
	if e.Operator.Type == queryparser.Like || e.Operator.Type == queryparser.NotLike {
		if e.Operand2.Type != queryparser.TypStr {
			return syncql.ErrorfLikeExpressionsRequireRhsString(db.GetContext(), "[%v]like expressions require right operand of type <string-literal>.", e.Operand2.Off)
		}
		// Compile the like pattern now to to check for errors.
		p, err := parseLikePattern(db, e.Operand2.Off, e.Operand2.Str, ec)
		if err != nil {
			return err
		}
		fixedPrefix, noWildcards := p.FixedPrefix()
		e.Operand2.Prefix = fixedPrefix
		// Optimization: If like/not like argument contains no wildcards, convert the expression to equals/not equals.
		if noWildcards {
			if e.Operator.Type == queryparser.Like {
				e.Operator.Type = queryparser.Equal
			} else { // not like
				e.Operator.Type = queryparser.NotEqual
			}
			// Since this is no longer a like expression, we need to unescape
			// any escaped chars.
			e.Operand2.Str = fixedPrefix
		}
		// Save the compiled pattern for later use in evaluation.
		e.Operand2.Pattern = p
	}

	// Is/IsNot expressions require operand1 to be a (value or function) and operand2 to be nil.
	if e.Operator.Type == queryparser.Is || e.Operator.Type == queryparser.IsNot {
		if !IsField(e.Operand1) && !IsFunction(e.Operand1) {
			return syncql.ErrorfIsIsNotRequireLhsValue(db.GetContext(), "[%v]'is/is not' expressions require left operand to be a value operand", e.Operand1.Off)
		}
		if e.Operand2.Type != queryparser.TypNil {
			return syncql.ErrorfIsIsNotRequireRhsNil(db.GetContext(), "[%v]'is/is not' expressions require right operand to be nil", e.Operand2.Off)
		}
	}

	// if an operand is k and the other operand is a literal, that literal must be a string
	// literal.
	if ContainsKeyOperand(e) && ((isLiteral(e.Operand1) && !isStringLiteral(e.Operand1)) ||
		(isLiteral(e.Operand2) && !isStringLiteral(e.Operand2))) {
		off := e.Operand1.Off
		if isLiteral(e.Operand2) {
			off = e.Operand2.Off
		}
		return syncql.ErrorfKeyExpressionLiteral(db.GetContext(), "[%v]key (i.e., 'k') compares against literals must be string literal", off)
	}

	// If either operand is a bool, only = and <> operators are allowed.
	if (e.Operand1.Type == queryparser.TypBool || e.Operand2.Type == queryparser.TypBool) && e.Operator.Type != queryparser.Equal && e.Operator.Type != queryparser.NotEqual {
		return syncql.ErrorfBoolInvalidExpression(db.GetContext(), "[%v]boolean operands may only be used in equals and not equals expressions", e.Operator.Off)
	}

	return nil
}

func checkOperand(db ds.Database, o *queryparser.Operand, ec *queryparser.EscapeClause) error {
	switch o.Type {
	case queryparser.TypExpr:
		return checkExpression(db, o.Expr, ec)
	case queryparser.TypField:
		switch o.Column.Segments[0].Value {
		case "k":
			if len(o.Column.Segments) > 1 {
				return syncql.ErrorfDotNotationDisallowedForKey(db.GetContext(), "[%v]dot notation may not be used on a key field", o.Column.Segments[1].Off)
			}
		case "v":
			// Nothing to do.
		case "K":
			// Be nice and warn of mistakenly capped 'K'.
			return syncql.ErrorfDidYouMeanLowercaseK(db.GetContext(), lowercaseKFormat, o.Column.Segments[0].Off)
		case "V":
			// Be nice and warn of mistakenly capped 'V'.
			return syncql.ErrorfDidYouMeanLowercaseV(db.GetContext(), lowercaseVFormat, o.Column.Segments[0].Off)
		default:
			return syncql.ErrorfBadFieldInWhere(db.GetContext(), "[%v]Where field must be 'k' or 'v[{.<ident>}...]'", o.Column.Segments[0].Off)
		}
		return nil
	case queryparser.TypFunction:
		// Each of the functions args needs to be checked first.
		for _, arg := range o.Function.Args {
			if err := checkOperand(db, arg, ec); err != nil {
				return err
			}
		}
		// Call queryfunctions.CheckFunction.  This will check for correct number of args
		// and, to the extent possible, correct types.
		// Furthermore, it may execute the function if the function takes no args or
		// takes only literal args (or an arg that is a function that is also executed
		// early).  CheckFunction will fill in arg types, return types and may fill in
		// Computed and RetValue.
		err := queryfunctions.CheckFunction(db, o.Function)
		if err != nil {
			return err
		}
		// If function executed early, computed will be true and RetValue set.
		// Convert the operand to the RetValue
		if o.Function.Computed {
			*o = *o.Function.RetValue
		}
	}
	return nil
}

func parseLikePattern(db ds.Database, off int64, s string, ec *queryparser.EscapeClause) (*pattern.Pattern, error) {
	escChar := '\x00' // nul is ignored as an escape char
	if ec != nil {
		escChar = ec.EscapeChar.Value
	}
	p, err := pattern.ParseWithEscapeChar(s, escChar)
	if err != nil {
		return nil, syncql.ErrorfInvalidLikePattern(db.GetContext(), "[%v]invalid like pattern: %v", off, err)
	}
	return p, nil
}

func IsLogicalOperator(o *queryparser.BinaryOperator) bool {
	return o.Type == queryparser.And || o.Type == queryparser.Or
}

func IsField(o *queryparser.Operand) bool {
	return o.Type == queryparser.TypField
}

func IsFunction(o *queryparser.Operand) bool {
	return o.Type == queryparser.TypFunction
}

func ContainsKeyOperand(expr *queryparser.Expression) bool {
	return IsKey(expr.Operand1) || IsKey(expr.Operand2)
}

func ContainsFieldOperand(f *queryparser.Field, expr *queryparser.Expression) bool {
	return IsExactField(f, expr.Operand1) || IsExactField(f, expr.Operand2)
}

func ContainsFunctionOperand(expr *queryparser.Expression) bool {
	return IsFunction(expr.Operand1) || IsFunction(expr.Operand2)
}

func ContainsValueFieldOperand(expr *queryparser.Expression) bool {
	return (expr.Operand1.Type == queryparser.TypField && IsValueField(expr.Operand1.Column)) ||
		(expr.Operand2.Type == queryparser.TypField && IsValueField(expr.Operand2.Column))

}

func isStringLiteral(o *queryparser.Operand) bool {
	return o.Type == queryparser.TypStr
}

func isLiteral(o *queryparser.Operand) bool {
	return o.Type == queryparser.TypBigInt ||
		o.Type == queryparser.TypBigRat || // currently, no way to specify as literal
		o.Type == queryparser.TypBool ||
		o.Type == queryparser.TypFloat ||
		o.Type == queryparser.TypInt ||
		o.Type == queryparser.TypStr ||
		o.Type == queryparser.TypTime || // currently, no way to specify as literal
		o.Type == queryparser.TypUint
}

func IsKey(o *queryparser.Operand) bool {
	return IsField(o) && IsKeyField(o.Column)
}

func IsExactField(f *queryparser.Field, o *queryparser.Operand) bool {
	if !IsField(o) {
		return false
	}
	oField := o.Column
	// Can't test for equality as offsets will be different.
	if len(f.Segments) != len(oField.Segments) {
		return false
	}
	for i := range f.Segments {
		if f.Segments[i].Value != oField.Segments[i].Value {
			return false
		}
	}
	return true
}

func IsKeyField(f *queryparser.Field) bool {
	return f.Segments[0].Value == "k"
}

func IsValueField(f *queryparser.Field) bool {
	return f.Segments[0].Value == "v"
}

func IsExpr(o *queryparser.Operand) bool {
	return o.Type == queryparser.TypExpr
}

func afterPrefix(prefix string) string {
	// Copied from syncbase.
	limit := []byte(prefix)
	for len(limit) > 0 {
		if limit[len(limit)-1] == 255 {
			limit = limit[:len(limit)-1] // chop off trailing \x00
		} else {
			limit[len(limit)-1]++ // add 1
			break                 // no carry
		}
	}
	return string(limit)
}

func computeStringFieldRangeForLike(prefix string) ds.StringFieldRange {
	if prefix == "" {
		return StringFieldRangeAll
	}
	return ds.StringFieldRange{Start: prefix, Limit: afterPrefix(prefix)}
}

func computeStringFieldRangesForNotLike(prefix string) *ds.StringFieldRanges {
	if prefix == "" {
		return &ds.StringFieldRanges{StringFieldRangeAll}
	}
	return &ds.StringFieldRanges{
		ds.StringFieldRange{Start: "", Limit: prefix},
		ds.StringFieldRange{Start: afterPrefix(prefix), Limit: ""},
	}
}

// The limit for a single value range is simply a zero byte appended.
func computeStringFieldRangeForSingleValue(start string) ds.StringFieldRange {
	limit := []byte(start)
	limit = append(limit, 0)
	return ds.StringFieldRange{Start: start, Limit: string(limit)}
}

// Compute a list of secondary index ranges to optionally be used by query's Table.Scan.
func CompileIndexRanges(idxField *queryparser.Field, kind vdl.Kind, where *queryparser.WhereClause) *ds.IndexRanges {
	var indexRanges ds.IndexRanges
	// Reconstruct field name from the segments in the field.
	sep := ""
	for _, seg := range idxField.Segments {
		indexRanges.FieldName += sep
		indexRanges.FieldName += seg.Value
		sep = "."
	}
	indexRanges.Kind = kind
	if where == nil {
		// Currently, only string is supported, so no need to check.
		indexRanges.StringRanges = &ds.StringFieldRanges{StringFieldRangeAll}
		indexRanges.NilAllowed = true
	} else {
		indexRanges.StringRanges = collectStringFieldRanges(idxField, where.Expr)
		indexRanges.NilAllowed = determineIfNilAllowed(idxField, where.Expr)
	}
	return &indexRanges
}

func computeRangeIntersection(lhs, rhs ds.StringFieldRange) *ds.StringFieldRange {
	// Detect if lhs.Start is contained within rhs or rhs.Start is contained within lhs.
	if (lhs.Start >= rhs.Start && compareStartToLimit(lhs.Start, rhs.Limit) < 0) ||
		(rhs.Start >= lhs.Start && compareStartToLimit(rhs.Start, lhs.Limit) < 0) {
		var start, limit string
		if lhs.Start < rhs.Start {
			start = rhs.Start
		} else {
			start = lhs.Start
		}
		if compareLimits(lhs.Limit, rhs.Limit) < 0 {
			limit = lhs.Limit
		} else {
			limit = rhs.Limit
		}
		return &ds.StringFieldRange{Start: start, Limit: limit}
	}
	return nil
}

func fieldRangeIntersection(lhs, rhs *ds.StringFieldRanges) *ds.StringFieldRanges {
	fieldRanges := &ds.StringFieldRanges{}
	lCur, rCur := 0, 0
	for lCur < len(*lhs) && rCur < len(*rhs) {
		// Any intersection at current cursors?
		if intersection := computeRangeIntersection((*lhs)[lCur], (*rhs)[rCur]); intersection != nil {
			// Add the intersection
			addStringFieldRange(*intersection, fieldRanges)
		}
		// increment the range with the lesser limit
		c := compareLimits((*lhs)[lCur].Limit, (*rhs)[rCur].Limit)
		switch c {
		case -1:
			lCur++
		case 1:
			rCur++
		default:
			lCur++
			rCur++
		}
	}
	return fieldRanges
}

func collectStringFieldRanges(idxField *queryparser.Field, expr *queryparser.Expression) *ds.StringFieldRanges { //nolint:gocyclo
	switch {
	case IsExpr(expr.Operand1): // then both operands must be expressions
		lhsStringFieldRanges := collectStringFieldRanges(idxField, expr.Operand1.Expr)
		rhsStringFieldRanges := collectStringFieldRanges(idxField, expr.Operand2.Expr)
		if expr.Operator.Type == queryparser.And {
			// intersection of lhsStringFieldRanges and rhsStringFieldRanges
			return fieldRangeIntersection(lhsStringFieldRanges, rhsStringFieldRanges)
		} // or
		// union of lhsStringFieldRanges and rhsStringFieldRanges
		for _, rhsStringFieldRange := range *rhsStringFieldRanges {
			addStringFieldRange(rhsStringFieldRange, lhsStringFieldRanges)
		}
		return lhsStringFieldRanges
	case ContainsFieldOperand(idxField, expr): // true if either operand is idxField
		switch {
		case IsField(expr.Operand1) && IsField(expr.Operand2):
			// <idx_field> <op> <idx_field>
			switch expr.Operator.Type {
			case queryparser.Equal, queryparser.GreaterThanOrEqual, queryparser.LessThanOrEqual:
				// True for all values of indexField
				return &ds.StringFieldRanges{StringFieldRangeAll}
			default: // queryparser.NotEqual, queryparser.GreaterThan, queryparser.LessThan:
				// False for all values of indexField
				return &ds.StringFieldRanges{}
			}
		case expr.Operator.Type == queryparser.Is:
			// <idx_field> is nil
			// False for entire range
			// TODO(jkline): Should the Scan contract return values where
			//               the index field is undefined?
			return &ds.StringFieldRanges{}
		case expr.Operator.Type == queryparser.IsNot:
			// k is not nil
			// True for all all values of indexField.
			return &ds.StringFieldRanges{StringFieldRangeAll}
		case isStringLiteral(expr.Operand2):
			// indexField <op> <string-literal>
			switch expr.Operator.Type {
			case queryparser.Equal:
				return &ds.StringFieldRanges{computeStringFieldRangeForSingleValue(expr.Operand2.Str)}
			case queryparser.GreaterThan:
				return &ds.StringFieldRanges{ds.StringFieldRange{Start: string(append([]byte(expr.Operand2.Str), 0)), Limit: MaxRangeLimit}}
			case queryparser.GreaterThanOrEqual:
				return &ds.StringFieldRanges{ds.StringFieldRange{Start: expr.Operand2.Str, Limit: MaxRangeLimit}}
			case queryparser.Like:
				return &ds.StringFieldRanges{computeStringFieldRangeForLike(expr.Operand2.Prefix)}
			case queryparser.NotLike:
				return computeStringFieldRangesForNotLike(expr.Operand2.Prefix)
			case queryparser.LessThan:
				return &ds.StringFieldRanges{ds.StringFieldRange{Start: "", Limit: expr.Operand2.Str}}
			case queryparser.LessThanOrEqual:
				return &ds.StringFieldRanges{ds.StringFieldRange{Start: "", Limit: string(append([]byte(expr.Operand2.Str), 0))}}
			default: // case queryparser.NotEqual:
				return &ds.StringFieldRanges{
					ds.StringFieldRange{Start: "", Limit: expr.Operand2.Str},
					ds.StringFieldRange{Start: string(append([]byte(expr.Operand2.Str), 0)), Limit: MaxRangeLimit},
				}
			}
		case isStringLiteral(expr.Operand1):
			// <string-literal> <op> k
			switch expr.Operator.Type {
			case queryparser.Equal:
				return &ds.StringFieldRanges{computeStringFieldRangeForSingleValue(expr.Operand1.Str)}
			case queryparser.GreaterThan:
				return &ds.StringFieldRanges{ds.StringFieldRange{Start: "", Limit: expr.Operand1.Str}}
			case queryparser.GreaterThanOrEqual:
				return &ds.StringFieldRanges{ds.StringFieldRange{Start: "", Limit: string(append([]byte(expr.Operand1.Str), 0))}}
			case queryparser.LessThan:
				return &ds.StringFieldRanges{ds.StringFieldRange{Start: string(append([]byte(expr.Operand1.Str), 0)), Limit: MaxRangeLimit}}
			case queryparser.LessThanOrEqual:
				return &ds.StringFieldRanges{ds.StringFieldRange{Start: expr.Operand1.Str, Limit: MaxRangeLimit}}
			default: // case queryparser.NotEqual:
				return &ds.StringFieldRanges{
					ds.StringFieldRange{Start: "", Limit: expr.Operand1.Str},
					ds.StringFieldRange{Start: string(append([]byte(expr.Operand1.Str), 0)), Limit: MaxRangeLimit},
				}
			}
		default:
			// A function or a field s being compared to the indexField;
			// or, an indexField is being compared to a literal which
			// is not a string.  The latter could be considered an error,
			// but for now, just allow the full range.
			return &ds.StringFieldRanges{StringFieldRangeAll}
		}
	default: // not a key compare, so it applies to the entire key range
		return &ds.StringFieldRanges{StringFieldRangeAll}
	}
}

func determineIfNilAllowed(idxField *queryparser.Field, expr *queryparser.Expression) bool {
	switch {
	case IsExpr(expr.Operand1): // then both operands must be expressions
		lhsNilAllowed := determineIfNilAllowed(idxField, expr.Operand1.Expr)
		rhsNilAllowed := determineIfNilAllowed(idxField, expr.Operand2.Expr)
		if expr.Operator.Type == queryparser.And {
			return lhsNilAllowed && rhsNilAllowed
		} // or
		return lhsNilAllowed || rhsNilAllowed
	case ContainsFieldOperand(idxField, expr): // true if either operand is idxField
		// The only way nil in the index field will evaluate to true is in the
		// Is Nil case.
		if expr.Operator.Type == queryparser.Is {
			// <idx_field> is nil
			return true
		}
		return false
	default: // not an index field expresion; as such, nil is allowed for the idx field
		return true
	}
}

// Helper function to compare start and limit byte arrays  taking into account that
// MaxRangeLimit is actually []byte{}.
func compareLimits(limitA, limitB string) int {
	switch {
	case limitA == limitB:
		return 0
	case limitA == MaxRangeLimit:
		return 1
	case limitB == MaxRangeLimit:
		return -1
	case limitA < limitB:
		return -1
	default:
		return 1
	}
}

func compareStartToLimit(startA, limitB string) int {
	switch {
	case limitB == MaxRangeLimit:
		return -1
	case startA == limitB:
		return 0
	case startA < limitB:
		return -1
	default:
		return 1
	}
}

func compareLimitToStart(limitA, startB string) int {
	switch {
	case limitA == MaxRangeLimit:
		return -1
	case limitA == startB:
		return 0
	case limitA < startB:
		return -1
	default:
		return 1
	}
}

func addStringFieldRange(fieldRange ds.StringFieldRange, fieldRanges *ds.StringFieldRanges) {
	handled := false
	// Is there an overlap with an existing range?
	for i, r := range *fieldRanges {
		// In the following if,
		// the first paren expr is true if the start of the range to be added is contained in r
		// the second paren expr is true if the limit of the range to be added is contained in r
		// the third paren expr is true if the range to be added entirely contains r
		if (fieldRange.Start >= r.Start && compareStartToLimit(fieldRange.Start, r.Limit) <= 0) ||
			(compareLimitToStart(fieldRange.Limit, r.Start) >= 0 && compareLimits(fieldRange.Limit, r.Limit) <= 0) ||
			(fieldRange.Start <= r.Start && compareLimits(fieldRange.Limit, r.Limit) >= 0) {

			// fieldRange overlaps with existing range at fieldRanges[i]
			// set newFieldRange to a range that ecompasses both
			var newFieldRange ds.StringFieldRange
			if fieldRange.Start < r.Start {
				newFieldRange.Start = fieldRange.Start
			} else {
				newFieldRange.Start = r.Start
			}
			if compareLimits(fieldRange.Limit, r.Limit) > 0 {
				newFieldRange.Limit = fieldRange.Limit
			} else {
				newFieldRange.Limit = r.Limit
			}
			// The new range may overlap with other ranges in fieldRanges
			// delete the current range and call addStringFieldRange again
			// This recursion will continue until no ranges overlap.
			*fieldRanges = append((*fieldRanges)[:i], (*fieldRanges)[i+1:]...)
			addStringFieldRange(newFieldRange, fieldRanges)
			handled = true // we don't want to add fieldRange below
			break
		}
	}
	// no overlap, just add it
	if !handled {
		*fieldRanges = append(*fieldRanges, fieldRange)
	}
	// sort before returning
	sort.Sort(*fieldRanges)
}

// Check escape clause. Escape char cannot be '\', ' ', or a wildcard.
// Return bool (true if escape char defined), escape char, error.
func checkEscapeClause(db ds.Database, e *queryparser.EscapeClause) error {
	if e == nil {
		return nil
	}
	switch ec := e.EscapeChar.Value; ec {
	case '\x00', '_', '%', ' ', '\\':
		return syncql.ErrorfInvalidEscapeChar(db.GetContext(), "[%v]'%v' is not a valid escape character", e.EscapeChar.Off, string(ec))
	default:
		return nil
	}
}

// Check limit clause.  Limit must be >= 1.
// Note: The parser will not allow negative numbers here.
func checkLimitClause(db ds.Database, l *queryparser.LimitClause) error {
	if l == nil {
		return nil
	}
	if l.Limit.Value < 1 {
		return syncql.ErrorfLimitMustBeGt0(db.GetContext(), "[%v]limit must be > 0.", l.Limit.Off)
	}
	return nil
}

// Check results offset clause.  Offset must be >= 0.
// Note: The parser will not allow negative numbers here, so this check is presently superfluous.
func checkResultsOffsetClause(db ds.Database, o *queryparser.ResultsOffsetClause) error {
	if o == nil {
		return nil
	}
	if o.ResultsOffset.Value < 0 {
		return syncql.ErrorfOffsetMustBeGe0(db.GetContext(), "[%v]offset must be > 0.", o.ResultsOffset.Off)
	}
	return nil
}
