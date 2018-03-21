package json

import "testing"

func TestQuote(t *testing.T) {
	for _, test := range []struct {
		input string
		want  string
	}{
		{``, `""`},
		{`""`, `""`},
		{"a", `"a"`},
		{"aa", `"aa"`},
		{"aaa", `"aaa"`},
		{`"a"`, `"a"`},
		{`"aa"`, `"aa"`},
		{`"aaa"`, `"aaa"`},
	} {
		got := quote(test.input)
		if got != test.want {
			t.Errorf("%q: got %q, want %q", test.input, got, test.want)
		}
	}
}
