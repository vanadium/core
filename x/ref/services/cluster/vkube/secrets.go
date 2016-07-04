// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"archive/tar"
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"text/template"
)

func createSecrets(secrets string, templates []string) error {
	files, err := decryptSecrets(secrets)
	if err != nil {
		return err
	}

	templateErrors := []error{}

	funcMap := template.FuncMap{
		"base64": func(name string) string {
			if v, exists := files[name]; exists {
				return v
			}
			templateErrors = append(templateErrors, fmt.Errorf("invalid name: %s", name))
			return "ERR"
		},
	}

	tmpl, err := template.New("").Funcs(funcMap).ParseFiles(templates...)
	if err != nil {
		return fmt.Errorf("template.ParseFiles failed: %v", err)
	}

	for _, t := range tmpl.Templates() {
		var buf bytes.Buffer
		if err := t.Execute(&buf, nil); err != nil {
			return fmt.Errorf("tmpl.Execute failed: %v", err)
		}
		if len(templateErrors) != 0 {
			return fmt.Errorf("template errors in %q: %v", t.Name(), templateErrors)
		}

		cmd := exec.Command(flagKubectlBin, "create", "-f", "-")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = &buf
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("kubectl command failed: %v", err)
		}
	}
	return nil
}

func decryptSecrets(secrets string) (map[string]string, error) {
	gpg := exec.Command(flagGpg, "-d", secrets)
	gpg.Stdin = os.Stdin
	gpg.Stderr = os.Stderr

	r, err := gpg.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("gpg.StdoutPipe failed: %v", err)
	}
	if err := gpg.Start(); err != nil {
		return nil, fmt.Errorf("gpg.Start failed: %v", err)
	}

	files := make(map[string]string)

	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("tr.Next failed: %v", err)
		}
		if hdr.FileInfo().IsDir() {
			continue
		}
		var buf bytes.Buffer
		if _, err := io.Copy(&buf, tr); err != nil {
			return nil, fmt.Errorf("io.Copy failed: %v", err)
		}
		name := strings.TrimPrefix(hdr.Name, "./")
		files[name] = base64.StdEncoding.EncodeToString(buf.Bytes())
	}

	if err := gpg.Wait(); err != nil {
		return nil, fmt.Errorf("gpg failed: %v", err)
	}

	return files, nil
}
