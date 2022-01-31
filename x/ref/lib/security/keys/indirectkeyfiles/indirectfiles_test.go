// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package indirectkeyfiles_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"v.io/x/ref/lib/security/keys"
	"v.io/x/ref/lib/security/keys/indirectkeyfiles"
)

const expected = `-----BEGIN VANADIUM INDIRECT PRIVATE KEY-----
Vanadium-Indirection-Type: filename

ZG9lc250LWV4aXN0
-----END VANADIUM INDIRECT PRIVATE KEY-----
`

var keyRegistrar = keys.NewRegistrar()

func init() {
	keys.MustRegister(keyRegistrar)
	indirectkeyfiles.MustRegister(keyRegistrar)
}

func TestIndirectionErrors(t *testing.T) {
	ctx := context.Background()

	bogus, err := indirectkeyfiles.MarshalPrivateKey([]byte("doesnt-exist"))
	if err != nil {
		t.Fatal(err)
	}

	if got, want := bogus, []byte(expected); !bytes.Equal(got, want) {
		t.Errorf("got %s want %s", got, want)
	}

	_, err = keyRegistrar.ParsePrivateKey(ctx, bogus, nil)
	if err == nil || !strings.Contains(err.Error(), "no such file or directory") {
		t.Errorf("unexpected or missing error %v", err)
	}

	tmpdir := t.TempDir()
	bogus, err = indirectkeyfiles.MarshalPrivateKey([]byte(tmpdir))
	if err != nil {
		t.Fatal(err)
	}
	_, err = keyRegistrar.ParsePrivateKey(ctx, bogus, nil)
	if err == nil || !strings.Contains(err.Error(), "is a directory") {
		t.Errorf("unexpected or missing error %v", err)
	}

	firstFile := filepath.Join(tmpdir, "first")
	secondFile := filepath.Join(tmpdir, "second")
	thirdFile := filepath.Join(tmpdir, "third")

	first, err := indirectkeyfiles.MarshalPrivateKey([]byte(secondFile))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(firstFile, first, 0600); err != nil {
		t.Fatal(err)
	}

	_, err = indirectkeyfiles.MarshalPrivateKey([]byte(thirdFile))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(secondFile, first, 0600); err != nil {
		t.Fatal(err)
	}

	_, err = keyRegistrar.ParsePrivateKey(ctx, first, nil)
	if err == nil || !strings.Contains(err.Error(), "indirection limit reached") {
		t.Errorf("unexpected or missing error %v", err)
	}
}
