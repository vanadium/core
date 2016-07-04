// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package writer

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"

	"v.io/v23/syncbase"
	"v.io/v23/vdl"
	vtime "v.io/v23/vdlroot/time"
)

type Justification int

const (
	Unknown Justification = iota
	Left
	Right
)

type FormattingWriter interface {
	Write(columnNames []string, rs syncbase.ResultStream) error
}

type tableWriter struct {
	w io.Writer
}

func NewTableWriter(w io.Writer) FormattingWriter {
	return &tableWriter{w}
}

// Write formats the results as ASCII tables.
func (t *tableWriter) Write(columnNames []string, rs syncbase.ResultStream) error {
	// Buffer the results so we can compute the column widths.
	columnWidths := make([]int, len(columnNames))
	for i, cName := range columnNames {
		columnWidths[i] = utf8.RuneCountInString(cName)
	}
	justification := make([]Justification, len(columnNames))
	var results [][]string
	for rs.Advance() {
		row := make([]string, len(columnNames))
		columnCount := rs.ResultCount()
		for i := 0; i != columnCount; i++ {
			var column *vdl.Value
			if err := rs.Result(i, &column); err != nil {
				return err
			}
			if i >= len(columnNames) {
				return errors.New("more columns in result than in columnNames")
			}
			if justification[i] == Unknown {
				justification[i] = getJustification(column)
			}
			columnStr := toString(column, false)
			row[i] = columnStr
			columnLen := utf8.RuneCountInString(columnStr)
			if columnLen > columnWidths[i] {
				columnWidths[i] = columnLen
			}
		}
		results = append(results, row)
	}
	if rs.Err() != nil {
		return rs.Err()
	}

	writeBorder(t.w, columnWidths)
	sep := "| "
	for i, cName := range columnNames {
		io.WriteString(t.w, fmt.Sprintf("%s%*s", sep, columnWidths[i], cName))
		sep = " | "
	}
	io.WriteString(t.w, " |\n")
	writeBorder(t.w, columnWidths)
	for _, result := range results {
		sep = "| "
		for i, column := range result {
			if justification[i] == Right {
				io.WriteString(t.w, fmt.Sprintf("%s%*s", sep, columnWidths[i], column))
			} else {
				io.WriteString(t.w, fmt.Sprintf("%s%-*s", sep, columnWidths[i], column))
			}
			sep = " | "
		}
		io.WriteString(t.w, " |\n")
	}
	writeBorder(t.w, columnWidths)
	return nil
}

func writeBorder(out io.Writer, columnWidths []int) {
	sep := "+-"
	for _, width := range columnWidths {
		io.WriteString(out, fmt.Sprintf("%s%s", sep, strings.Repeat("-", width)))
		sep = "-+-"
	}
	io.WriteString(out, "-+\n")
}

func getJustification(val *vdl.Value) Justification {
	switch val.Kind() {
	// TODO(kash): Floating point numbers should have the decimal point line up.
	case vdl.Bool, vdl.Byte, vdl.Uint16, vdl.Uint32, vdl.Uint64, vdl.Int8,
		vdl.Int16, vdl.Int32, vdl.Int64, vdl.Float32, vdl.Float64:
		return Right
	// TODO(kash): Leave nil values as unknown.
	default:
		return Left
	}
}

type csvWriter struct {
	w         io.Writer
	delimiter string
}

func NewCSVWriter(w io.Writer, delimiter string) FormattingWriter {
	return &csvWriter{w, delimiter}
}

// Write formats the results as CSV as specified by https://tools.ietf.org/html/rfc4180.
func (c *csvWriter) Write(columnNames []string, rs syncbase.ResultStream) error {
	delim := ""
	for _, cName := range columnNames {
		str := doubleQuoteForCSV(cName, c.delimiter)
		io.WriteString(c.w, fmt.Sprintf("%s%s", delim, str))
		delim = c.delimiter
	}
	io.WriteString(c.w, "\n")
	for rs.Advance() {
		delim := ""
		for i, n := 0, rs.ResultCount(); i != n; i++ {
			var column *vdl.Value
			if err := rs.Result(i, &column); err != nil {
				return err
			}
			str := doubleQuoteForCSV(toString(column, false), c.delimiter)
			io.WriteString(c.w, fmt.Sprintf("%s%s", delim, str))
			delim = c.delimiter
		}
		io.WriteString(c.w, "\n")
	}
	return rs.Err()
}

// doubleQuoteForCSV follows the escaping rules from
// https://tools.ietf.org/html/rfc4180. In particular, values containing
// newlines, double quotes, and the delimiter must be enclosed in double
// quotes.
func doubleQuoteForCSV(str, delimiter string) string {
	doubleQuote := strings.Index(str, delimiter) != -1 || strings.Index(str, "\n") != -1
	if strings.Index(str, "\"") != -1 {
		str = strings.Replace(str, "\"", "\"\"", -1)
		doubleQuote = true
	}
	if doubleQuote {
		str = "\"" + str + "\""
	}
	return str
}

type jsonWriter struct {
	w io.Writer
}

func NewJSONWriter(w io.Writer) FormattingWriter {
	return &jsonWriter{w}
}

// Write formats the result as a JSON array of arrays (rows) of values.
func (j *jsonWriter) Write(columnNames []string, rs syncbase.ResultStream) error {
	io.WriteString(j.w, "[")
	jsonColNames := make([][]byte, len(columnNames))
	for i, cName := range columnNames {
		jsonCName, err := json.Marshal(cName)
		if err != nil {
			panic(fmt.Sprintf("JSON marshalling failed for column name: %v", err))
		}
		jsonColNames[i] = jsonCName
	}
	bOpen := "{"
	for rs.Advance() {
		io.WriteString(j.w, bOpen)
		linestart := "\n  "
		for i, n := 0, rs.ResultCount(); i != n; i++ {
			var column *vdl.Value
			if err := rs.Result(i, &column); err != nil {
				return err
			}
			str := toJson(column)
			io.WriteString(j.w, fmt.Sprintf("%s%s: %s", linestart, jsonColNames[i], str))
			linestart = ",\n  "
		}
		io.WriteString(j.w, "\n}")
		bOpen = ", {"
	}
	io.WriteString(j.w, "]\n")
	return rs.Err()
}

// Converts VDL value to readable yet parseable string representation.
// If nested is not set, strings outside composites are left unquoted.
// TODO(ivanpi): Handle cycles and improve non-tree DAG handling.
func toString(val *vdl.Value, nested bool) string {
	switch val.Type() {
	case vdl.TypeOf(vtime.Time{}), vdl.TypeOf(vtime.Duration{}):
		s, err := toStringNative(val)
		if err != nil {
			panic(fmt.Sprintf("toStringNative failed for builtin time type: %v", err))
		}
		if nested {
			s = strconv.Quote(s)
		}
		return s
	default:
		// fall through to Kind switch
	}
	switch val.Kind() {
	case vdl.Bool:
		return fmt.Sprint(val.Bool())
	case vdl.Byte, vdl.Uint16, vdl.Uint32, vdl.Uint64:
		return fmt.Sprint(val.Uint())
	case vdl.Int8, vdl.Int16, vdl.Int32, vdl.Int64:
		return fmt.Sprint(val.Int())
	case vdl.Float32, vdl.Float64:
		return fmt.Sprint(val.Float())
	case vdl.String:
		s := val.RawString()
		if nested {
			s = strconv.Quote(s)
		}
		return s
	case vdl.Enum:
		return val.EnumLabel()
	case vdl.Array, vdl.List:
		return listToString("[", ", ", "]", val.Len(), func(i int) string {
			return toString(val.Index(i), true)
		})
	case vdl.Any, vdl.Optional:
		if val.IsNil() {
			if nested {
				return "nil"
			}
			// TODO(ivanpi): Blank is better for CSV, but <nil> might be better for table and TSV.
			return ""
		}
		return toString(val.Elem(), nested)
	case vdl.Struct:
		return listToString("{", ", ", "}", val.Type().NumField(), func(i int) string {
			field := toString(val.StructField(i), true)
			return fmt.Sprintf("%s: %s", val.Type().Field(i).Name, field)
		})
	case vdl.Union:
		ui, uv := val.UnionField()
		field := toString(uv, true)
		return fmt.Sprintf("%s: %s", val.Type().Field(ui).Name, field)
	case vdl.Set:
		// TODO(ivanpi): vdl.SortValuesAsString() used for predictable output ordering.
		// Use a more sensible sort for numbers etc.
		keys := vdl.SortValuesAsString(val.Keys())
		return listToString("{", ", ", "}", len(keys), func(i int) string {
			return toString(keys[i], true)
		})
	case vdl.Map:
		// TODO(ivanpi): vdl.SortValuesAsString() used for predictable output ordering.
		// Use a more sensible sort for numbers etc.
		keys := vdl.SortValuesAsString(val.Keys())
		return listToString("{", ", ", "}", len(keys), func(i int) string {
			k := toString(keys[i], true)
			v := toString(val.MapIndex(keys[i]), true)
			return fmt.Sprintf("%s: %s", k, v)
		})
	case vdl.TypeObject:
		return val.String()
	default:
		panic(fmt.Sprintf("unknown Kind %s", val.Kind()))
	}
}

// Converts a VDL value to string using the corresponding native type String()
// method.
func toStringNative(val *vdl.Value) (string, error) {
	var natVal interface{}
	if err := vdl.Convert(&natVal, val); err != nil {
		return "", fmt.Errorf("failed converting %s to native value: %v", val.Type().String(), err)
	}
	if _, ok := natVal.(*vdl.Value); ok {
		return "", fmt.Errorf("failed converting %s to native value: got vdl.Value", val.Type().String())
	}
	if strNatVal, ok := natVal.(fmt.Stringer); !ok {
		return "", fmt.Errorf("native value of %s doesn't implement String()", val.Type().String())
	} else {
		return strNatVal.String(), nil
	}
}

// Stringifies a sequence of n elements, where element i string representation
// is obtained using elemToString(i),
func listToString(begin, sep, end string, n int, elemToString func(i int) string) string {
	elems := make([]string, n)
	for i, _ := range elems {
		elems[i] = elemToString(i)
	}
	return begin + strings.Join(elems, sep) + end
}

// Converts VDL value to JSON representation.
func toJson(val *vdl.Value) string {
	jf := toJsonFriendly(val)
	jOut, err := json.Marshal(jf)
	if err != nil {
		panic(fmt.Sprintf("JSON marshalling failed: %v", err))
	}
	return string(jOut)
}

// Converts VDL value to Go type compatible with json.Marshal().
func toJsonFriendly(val *vdl.Value) interface{} {
	switch val.Type() {
	case vdl.TypeOf(vtime.Time{}), vdl.TypeOf(vtime.Duration{}):
		s, err := toStringNative(val)
		if err != nil {
			panic(fmt.Sprintf("toStringNative failed for builtin time type: %v", err))
		}
		return s
	default:
		// fall through to Kind switch
	}
	switch val.Kind() {
	case vdl.Bool:
		return val.Bool()
	case vdl.Byte, vdl.Uint16, vdl.Uint32, vdl.Uint64:
		return val.Uint()
	case vdl.Int8, vdl.Int16, vdl.Int32, vdl.Int64:
		return val.Int()
	case vdl.Float32, vdl.Float64:
		return val.Float()
	case vdl.String:
		return val.RawString()
	case vdl.Enum:
		return val.EnumLabel()
	case vdl.Array, vdl.List:
		arr := make([]interface{}, val.Len())
		for i, _ := range arr {
			arr[i] = toJsonFriendly(val.Index(i))
		}
		return arr
	case vdl.Any, vdl.Optional:
		if val.IsNil() {
			return nil
		}
		return toJsonFriendly(val.Elem())
	case vdl.Struct:
		// TODO(ivanpi): Consider lowercasing field names.
		return toOrderedMap(val.Type().NumField(), func(i int) (string, interface{}) {
			return val.Type().Field(i).Name, toJsonFriendly(val.StructField(i))
		})
	case vdl.Union:
		// TODO(ivanpi): Consider lowercasing field name.
		ui, uv := val.UnionField()
		return toOrderedMap(1, func(_ int) (string, interface{}) {
			return val.Type().Field(ui).Name, toJsonFriendly(uv)
		})
	case vdl.Set:
		// TODO(ivanpi): vdl.SortValuesAsString() used for predictable output ordering.
		// Use a more sensible sort for numbers etc.
		keys := vdl.SortValuesAsString(val.Keys())
		return toOrderedMap(len(keys), func(i int) (string, interface{}) {
			return toString(keys[i], false), true
		})
	case vdl.Map:
		// TODO(ivanpi): vdl.SortValuesAsString() used for predictable output ordering.
		// Use a more sensible sort for numbers etc.
		keys := vdl.SortValuesAsString(val.Keys())
		return toOrderedMap(len(keys), func(i int) (string, interface{}) {
			return toString(keys[i], false), toJsonFriendly(val.MapIndex(keys[i]))
		})
	case vdl.TypeObject:
		return val.String()
	default:
		panic(fmt.Sprintf("unknown Kind %s", val.Kind()))
	}
}

// Serializes to JSON object, preserving key order.
// Native Go map will serialize to JSON object with sorted keys, which is
// unexpected behaviour for a struct.
type orderedMap []orderedMapElem

type orderedMapElem struct {
	Key string
	Val interface{}
}

var _ json.Marshaler = (*orderedMap)(nil)

// Builds an orderedMap with n elements, obtaining the key and value of element
// i using elemToKeyVal(i).
func toOrderedMap(n int, elemToKeyVal func(i int) (string, interface{})) orderedMap {
	om := make(orderedMap, n)
	for i, _ := range om {
		om[i].Key, om[i].Val = elemToKeyVal(i)
	}
	return om
}

// Serializes orderedMap to JSON object, preserving key order.
func (om orderedMap) MarshalJSON() (_ []byte, rerr error) {
	defer func() {
		if r := recover(); r != nil {
			rerr = fmt.Errorf("orderedMap: %v", r)
		}
	}()
	return []byte(listToString("{", ",", "}", len(om), func(i int) string {
		keyJson, err := json.Marshal(om[i].Key)
		if err != nil {
			panic(err)
		}
		valJson, err := json.Marshal(om[i].Val)
		if err != nil {
			panic(err)
		}
		return fmt.Sprintf("%s:%s", keyJson, valJson)
	})), nil
}
