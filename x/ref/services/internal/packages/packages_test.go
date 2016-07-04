// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package packages_test

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"v.io/v23/services/repository"

	"v.io/x/ref/services/internal/packages"
)

func TestInstall(t *testing.T) {
	workdir, err := ioutil.TempDir("", "packages-test-")
	if err != nil {
		t.Fatalf("ioutil.TempDir failed: %v", err)
	}
	defer os.RemoveAll(workdir)
	srcdir := filepath.Join(workdir, "src")
	dstdir := filepath.Join(workdir, "dst")
	createFiles(t, srcdir)

	zipfile := filepath.Join(workdir, "archivezip")
	tarfile := filepath.Join(workdir, "archivetar")
	tgzfile := filepath.Join(workdir, "archivetgz")

	makeZip(t, zipfile, srcdir)
	makeTar(t, tarfile, srcdir)
	doGzip(t, tarfile, tgzfile)

	binfile := filepath.Join(workdir, "binfile")
	ioutil.WriteFile(binfile, []byte("This is a binary file"), os.FileMode(0644))
	ioutil.WriteFile(packages.MediaInfoFile(binfile), []byte(`{"type":"application/octet-stream"}`), os.FileMode(0644))

	expected := []string{
		"a perm:700",
		"a/b perm:700",
		"a/b/xyzzy.txt perm:600",
		"a/bar.txt perm:600",
		"a/foo.txt perm:600",
	}
	for _, file := range []string{zipfile, tarfile, tgzfile} {
		if err := packages.Install(file, dstdir); err != nil {
			t.Errorf("packages.Install failed for %q: %v", file, err)
		}
		files := scanDir(dstdir)
		if !reflect.DeepEqual(files, expected) {
			t.Errorf("unexpected result for %q: got %q, want %q", file, files, expected)
		}
		if err := os.RemoveAll(dstdir); err != nil {
			t.Fatalf("os.RemoveAll(%q) failed: %v", dstdir, err)
		}
	}
	dstfile := filepath.Join(workdir, "dstfile")
	if err := packages.Install(binfile, dstfile); err != nil {
		t.Errorf("packages.Install failed for %q: %v", binfile, err)
	}
	contents, err := ioutil.ReadFile(dstfile)
	if err != nil {
		t.Errorf("ReadFile(%q) failed: %v", dstfile, err)
	}
	if want, got := "This is a binary file", string(contents); want != got {
		t.Errorf("unexpected result for %q: got %q, want %q", binfile, got, want)
	}
}

func TestMediaInfo(t *testing.T) {
	testcases := []struct {
		filename string
		expected repository.MediaInfo
	}{
		{"foo.zip", repository.MediaInfo{Type: "application/zip"}},
		{"foo.ZIP", repository.MediaInfo{Type: "application/zip"}},
		{"foo.tar", repository.MediaInfo{Type: "application/x-tar"}},
		{"foo.TAR", repository.MediaInfo{Type: "application/x-tar"}},
		{"foo.tgz", repository.MediaInfo{Type: "application/x-tar", Encoding: "gzip"}},
		{"FOO.TAR.GZ", repository.MediaInfo{Type: "application/x-tar", Encoding: "gzip"}},
		{"foo.tbz2", repository.MediaInfo{Type: "application/x-tar", Encoding: "bzip2"}},
		{"foo.tar.bz2", repository.MediaInfo{Type: "application/x-tar", Encoding: "bzip2"}},
		{"foo", repository.MediaInfo{Type: "application/octet-stream"}},
	}
	for _, tc := range testcases {
		if got := packages.MediaInfoForFileName(tc.filename); !reflect.DeepEqual(got, tc.expected) {
			t.Errorf("unexpected result for %q: got %v, want %v", tc.filename, got, tc.expected)
		}
	}
}

func createFiles(t *testing.T, dir string) {
	if err := os.Mkdir(dir, os.FileMode(0755)); err != nil {
		t.Fatalf("os.Mkdir(%q) failed: %v", dir, err)
	}
	dirs := []string{"a", "a/b"}
	for _, d := range dirs {
		fullname := filepath.Join(dir, d)
		if err := os.Mkdir(fullname, os.FileMode(0755)); err != nil {
			t.Fatalf("os.Mkdir(%q) failed: %v", fullname, err)
		}
	}
	files := []string{"a/foo.txt", "a/bar.txt", "a/b/xyzzy.txt"}
	for _, f := range files {
		fullname := filepath.Join(dir, f)
		if err := ioutil.WriteFile(fullname, []byte(f), os.FileMode(0644)); err != nil {
			t.Fatalf("ioutil.WriteFile(%q) failed: %v", fullname, err)
		}
	}
}

func makeZip(t *testing.T, zipfile, dir string) {
	if err := packages.CreateZip(zipfile, dir); err != nil {
		t.Fatalf("packages.CreateZip failed: %v", err)
	}
}

func makeTar(t *testing.T, tarfile, dir string) {
	tf, err := os.OpenFile(tarfile, os.O_CREATE|os.O_WRONLY, os.FileMode(0644))
	if err != nil {
		t.Fatalf("os.OpenFile(%q) failed: %v", tarfile, err)
	}
	defer tf.Close()

	tw := tar.NewWriter(tf)
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			t.Fatalf("Walk(%q) error: %v", dir, err)
		}
		if dir == path {
			return nil
		}
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			t.Fatalf("tar.FileInfoHeader failed: %v", err)
		}
		hdr.Name, _ = filepath.Rel(dir, path)
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("tw.WriteHeader failed: %v", err)
		}
		if !info.IsDir() {
			content, err := ioutil.ReadFile(path)
			if err != nil {
				t.Fatalf("ioutil.ReadFile(%q) failed: %v", path, err)
			}
			if _, err := tw.Write(content); err != nil {
				t.Fatalf("tw.Write failed: %v", err)
			}
		}
		return nil
	})
	if err := tw.Close(); err != nil {
		t.Fatalf("tw.Close failed: %v", err)
	}
	if err := ioutil.WriteFile(packages.MediaInfoFile(tarfile), []byte(`{"type":"application/x-tar"}`), os.FileMode(0644)); err != nil {
		t.Fatalf("ioutil.WriteFile() failed: %v", err)
	}
}

func doGzip(t *testing.T, infile, outfile string) {
	in, err := os.Open(infile)
	if err != nil {
		t.Fatalf("os.Open(%q) failed: %v", infile, err)
	}
	defer in.Close()
	out, err := os.OpenFile(outfile, os.O_CREATE|os.O_WRONLY, os.FileMode(0644))
	if err != nil {
		t.Fatalf("os.OpenFile(%q) failed: %v", outfile, err)
	}
	defer out.Close()
	writer := gzip.NewWriter(out)
	defer writer.Close()
	if _, err := io.Copy(writer, in); err != nil {
		t.Fatalf("io.Copy() failed: %v", err)
	}

	info, err := packages.LoadMediaInfo(infile)
	if err != nil {
		t.Fatalf("LoadMediaInfo(%q) failed: %v", infile, err)
	}
	info.Encoding = "gzip"
	if err := packages.SaveMediaInfo(outfile, info); err != nil {
		t.Fatalf("SaveMediaInfo(%v) failed: %v", outfile, err)
	}
}

func scanDir(root string) []string {
	files := []string{}
	filepath.Walk(root, func(path string, info os.FileInfo, _ error) error {
		if root == path {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		perm := info.Mode() & 0700
		files = append(files, fmt.Sprintf("%s perm:%o", rel, perm))
		return nil
	})
	sort.Strings(files)
	return files
}
