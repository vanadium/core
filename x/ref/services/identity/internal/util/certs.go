// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package util

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"v.io/x/lib/envvar"
)

// WriteCertAndKey creates a certificate and private key for a given host and
// duration and writes them to cert.pem and key.pem in tmpdir.  It returns the
// locations of the files, or an error if one is encountered.
func WriteCertAndKey(host string, duration time.Duration) (string, string, error) {
	listCmd := exec.Command("go", "list", "-f", "{{.Dir}}", "crypto/tls")
	output, err := listCmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("%s failed: %v", strings.Join(listCmd.Args, " "), err)
	}
	tmpDir := os.TempDir()
	generateCertFile := filepath.Join(strings.TrimSpace(string(output)), "generate_cert.go")
	generateCertCmd := exec.Command("go", "run", generateCertFile, "--host", host, "--duration", duration.String())
	// go run will fail if run from outside of a go tree with go.mod if
	// GO111MODULE is enabled.
	env := envvar.SliceToMap(os.Environ())
	env["GO111MODULE"] = "off"
	generateCertCmd.Env = envvar.MapToSlice(env)
	generateCertCmd.Dir = tmpDir
	if output, err := generateCertCmd.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "%v failed:\n%s\n", generateCertCmd.Args, output)
		return "", "", fmt.Errorf("Could not generate key and cert: %v", err)
	}
	return filepath.Join(tmpDir, "cert.pem"), filepath.Join(tmpDir, "key.pem"), nil
}
