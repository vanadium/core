// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vom

import (
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"
	"testing/iotest"

	"v.io/v23/vdl"
)

// ReadMode ensures the decoder handles short reads and different EOF semantics.
type ReadMode int

const (
	ReadAll        ReadMode = iota // Read fills all data,  EOF after final data
	ReadHalf                       // Read fills half data, EOF after final data
	ReadOneByte                    // Read fills one byte,  EOF after final data
	ReadAllEOF                     // Read fills all data,  EOF with final data
	ReadHalfEOF                    // Read fills half data, EOF with final data
	ReadOneByteEOF                 // Read fills one byte,  EOF with final data
)

var AllReadModes = [...]ReadMode{ReadAll, ReadHalf, ReadOneByte, ReadAllEOF, ReadHalfEOF, ReadOneByteEOF}

func (m ReadMode) String() string {
	switch m {
	case ReadAll:
		return "ReadAll"
	case ReadHalf:
		return "ReadHalf"
	case ReadOneByte:
		return "ReadOneByte"
	case ReadAllEOF:
		return "ReadAllEOF"
	case ReadHalfEOF:
		return "ReadHalfEOF"
	case ReadOneByteEOF:
		return "ReadOneByteEOF"
	default:
		panic(fmt.Errorf("unknown ReadMode %d", m))
	}
}

func (m ReadMode) TestReader(r io.Reader) io.Reader {
	switch m {
	case ReadAll:
		return r
	case ReadHalf:
		return iotest.HalfReader(r)
	case ReadOneByte:
		return iotest.OneByteReader(r)
	case ReadAllEOF:
		return iotest.DataErrReader(r)
	case ReadHalfEOF:
		return iotest.DataErrReader(iotest.HalfReader(r))
	case ReadOneByteEOF:
		return iotest.DataErrReader(iotest.OneByteReader(r))
	default:
		panic(fmt.Errorf("unknown ReadMode %d", m))
	}
}

// ABCReader returns data looping from a-z, up to lim bytes.
func ABCReader(lim int) io.Reader {
	return &abcRead{lim: lim}
}

type abcRead struct {
	n, lim int
}

func (abc *abcRead) Read(p []byte) (int, error) {
	if abc.n >= abc.lim {
		return 0, io.EOF
	}
	startlen := len(p)
	for ; len(p) > 0 && abc.n < abc.lim; abc.n++ {
		p[0] = byte('a' + (abc.n % 26))
		p = p[1:]
	}
	return startlen - len(p), nil
}

func ABCBytes(lim int) []byte {
	b := make([]byte, lim)
	io.ReadFull(ABCReader(lim), b) //nolint:errcheck
	return b
}

// matchHexPat compares the given target and pat hex codes, and returns true if
// they match.  We allow special syntax in the pat code; in addition to regular
// string matching, we allow sequences that may appear in any order.
// E.g. "1122[33,44]55" means that 33 and 44 are a sequence that may appear in
// any order, so either "1122334455" or "1122443355" are accepted.
//
// We allow this special syntax since parts of the encoding aren't
// deterministic; e.g. Go maps are unordered.
func matchHexPat(target, pat string) (bool, error) {
	orig := pat
	for pat != "" {
		start := strings.IndexRune(pat, '[')
		// If there isn't a start token, just compare the full strings.
		if start == -1 {
			return target == pat, nil
		}
		// Compare everything up to the start token.
		if !strings.HasPrefix(target, pat[:start]) {
			return false, nil
		}
		// Now compare all permutations of the sequence until we find a match.
		pat = pat[start+1:] // remove '[' too
		target = target[start:]
		end := strings.IndexRune(pat, ']')
		if end == -1 {
			return false, fmt.Errorf("Malformed hex pattern, no closing ] in %q", orig)
		}
		seqs := strings.Split(pat[:end], ",")
		if !matchPrefixSeq(target, seqs) {
			return false, nil
		}
		// Found a match, move past this sequence.  An example of our state:
		//    pat="11,22]3344" target="22113344" end_seq=5
		// We need to remove everything up to and including "]" from pat, and
		// remove the matched sequence length from target, so that we get:
		//    pat="3344" target="3344"
		pat = pat[end+1:]
		target = target[end-len(seqs)+1:]
	}
	return target == "", nil
}

// matchPrefixSeq is a recursive function that returns true iff a prefix of the
// target string matches any permutation of the seqs strings.
func matchPrefixSeq(target string, seqs []string) bool {
	if len(seqs) == 0 {
		return true
	}
	for ix, seq := range seqs {
		if strings.HasPrefix(target, seq) {
			if matchPrefixSeq(target[len(seq):], append(seqs[:ix], seqs[ix+1:]...)) {
				return true
			}
		}
	}
	return false
}

// binFromHexPat returns a binary string based on the given hex pattern.
// Allowed hex patterns are the same as for matchHexPat.
//nolint:deadcode,unused
func binFromHexPat(pat string) (string, error) {
	if len(pat) == 0 {
		return "", nil
	}
	// TODO(toddw): We could also choose to randomly re-order the sequences.
	hex := strings.NewReplacer("[", "", "]", "", ",", "").Replace(pat)
	var bin []byte
	_, err := fmt.Sscanf(hex, "%x", &bin)
	return string(bin), err
}

func TestMatchPrefixSeq(t *testing.T) {
	tests := []struct {
		target string
		seqs   []string
		expect bool
	}{
		{"112233", []string{"11"}, true},
		{"112233", []string{"1122"}, true},
		{"112233", []string{"11", "22"}, true},
		{"112233", []string{"22", "11"}, true},
		{"112233", []string{"112233"}, true},
		{"112233", []string{"11", "2233"}, true},
		{"112233", []string{"2233", "11"}, true},
		{"112233", []string{"112", "233"}, true},
		{"112233", []string{"233", "112"}, true},
		{"112233", []string{"1122", "33"}, true},
		{"112233", []string{"33", "1122"}, true},
		{"112233", []string{"1", "1223", "3"}, true},
		{"112233", []string{"3", "1223", "1"}, true},
		{"112233", []string{"11", "22", "33"}, true},
		{"112233", []string{"11", "33", "22"}, true},
		{"112233", []string{"22", "11", "33"}, true},
		{"112233", []string{"22", "33", "11"}, true},
		{"112233", []string{"33", "11", "22"}, true},
		{"112233", []string{"33", "22", "11"}, true},
		{"112233", []string{"1", "1", "2", "2", "3", "3"}, true},
		{"112233", []string{"1", "2", "3", "1", "2", "3"}, true},
		{"112233", []string{"332211"}, false},
		{"112233", []string{"1122333"}, false},
		{"112233", []string{"11", "22333"}, false},
		{"112233", []string{"11", "22", "333"}, false},
		{"112233", []string{"11", "11", "11"}, false},
	}
	for _, test := range tests {
		if matchPrefixSeq(test.target, test.seqs) != test.expect {
			t.Errorf("matchPrefixSeq(%q, %v) != %v", test.target, test.seqs, test.expect)
		}
	}
}

func TestMatchHexPat(t *testing.T) {
	tests := []struct {
		target, pat string
		expect      bool
	}{
		{"112233", "112233", true},
		{"112233", "[112233]", true},
		{"112233", "11[2233]", true},
		{"112233", "1122[33]", true},
		{"112233", "11[22]33", true},
		{"112233", "[11,22]33", true},
		{"112233", "[22,11]33", true},
		{"112233", "11[22,33]", true},
		{"112233", "11[33,22]", true},
		{"112233", "1[12,23]3", true},
		{"112233", "1[23,12]3", true},
		{"112233", "[11,22,33]", true},
		{"112233", "[11,33,22]", true},
		{"112233", "[22,11,33]", true},
		{"112233", "[22,33,11]", true},
		{"112233", "[33,11,22]", true},
		{"112233", "[33,22,11]", true},

		{"112233", "11223", false},
		{"112233", "1122333", false},
		{"112233", "[11223]", false},
		{"112233", "[1122333]", false},
		{"112233", "11[223]", false},
		{"112233", "11[22333]", false},
		{"112233", "11[22,3]", false},
		{"112233", "11[223,33]", false},
		{"112233", "[11,2]33", false},
		{"112233", "[22,1]33", false},
		{"112233", "11[2,3]33", false},
		{"112233", "[11,2,33]", false},
	}
	for _, test := range tests {
		actual, err := matchHexPat(test.target, test.pat)
		if err != nil {
			t.Error(err)
		}
		if actual != test.expect {
			t.Errorf("matchHexPat(%q, %q) != %v", test.target, test.pat, test.expect)
		}
	}
}

//nolint:deadcode,unused
func toGoValue(value *vdl.Value) (interface{}, error) {
	if value == nil {
		return nil, nil
	}
	if value.Kind() == vdl.Any {
		return nil, nil
	}
	rt := vdl.TypeToReflect(value.Type())
	if rt == nil {
		return reflect.Value{}, fmt.Errorf("TypeToReflect(%v) failed", value.Type())
	}
	rv := reflect.New(rt)
	if err := vdl.Convert(rv.Interface(), value); err != nil {
		return reflect.Value{}, fmt.Errorf("vdl.Convert(%T, %v) failed: %v", rt, value, err)
	}
	return rv.Elem().Interface(), nil
}
