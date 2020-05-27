// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package packages provides functionality to install ZIP and TAR packages.
package packages

import (
	"archive/tar"
	"archive/zip"
	"compress/bzip2"
	"compress/gzip"
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"v.io/v23/services/repository"
	"v.io/v23/verror"
)

const (
	defaultType    = "application/octet-stream"
	createDirMode  = 0755
	createFileMode = 0644
)

var typemap = map[string]repository.MediaInfo{
	".zip":     {Type: "application/zip"},
	".tar":     {Type: "application/x-tar"},
	".tgz":     {Type: "application/x-tar", Encoding: "gzip"},
	".tar.gz":  {Type: "application/x-tar", Encoding: "gzip"},
	".tbz2":    {Type: "application/x-tar", Encoding: "bzip2"},
	".tb2":     {Type: "application/x-tar", Encoding: "bzip2"},
	".tbz":     {Type: "application/x-tar", Encoding: "bzip2"},
	".tar.bz2": {Type: "application/x-tar", Encoding: "bzip2"},
}

const pkgPath = "v.io/x/ref/services/internal/packages"

var (
	errBadMediaType    = verror.Register(pkgPath+".errBadMediaType", verror.NoRetry, "{1:}{2:} unsupported media type{:_}")
	errMkDirFailed     = verror.Register(pkgPath+".errMkDirFailed", verror.NoRetry, "{1:}{2:} os.Mkdir({3}) failed{:_}")
	errFailedToExtract = verror.Register(pkgPath+".errFailedToExtract", verror.NoRetry, "{1:}{2:} failed to extract file {3} outside of install directory{:_}")
	errBadFileSize     = verror.Register(pkgPath+".errBadFileSize", verror.NoRetry, "{1:}{2:} file size doesn't match for {3}: {4} != {5}{:_}")
	errBadEncoding     = verror.Register(pkgPath+".errBadEncoding", verror.NoRetry, "{1:}{2:} unsupported encoding{:_}")
)

// MediaInfoFile returns the name of the file where the media info is stored for
// the given package file.
func MediaInfoFile(pkgFile string) string {
	const mediaInfoFileSuffix = ".__info"
	return pkgFile + mediaInfoFileSuffix
}

// MediaInfoForFileName returns the MediaInfo based on the file's extension.
func MediaInfoForFileName(fileName string) repository.MediaInfo {
	fileName = strings.ToLower(fileName)
	for k, v := range typemap {
		if strings.HasSuffix(fileName, k) {
			return v
		}
	}
	return repository.MediaInfo{Type: defaultType}
}

func copyFile(src, dst string) error {
	s, err := os.Open(src)
	if err != nil {
		return err
	}
	defer s.Close()
	d, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY, createFileMode)
	if err != nil {
		return err
	}
	defer d.Close()
	if _, err = io.Copy(d, s); err != nil {
		return err
	}
	return d.Sync()
}

// Install installs a package in the given destination. If the package is a TAR
// or ZIP archive, the destination becomes a directory where the archive content
// is extracted.  Otherwise, the destination is hard-linked to the package (or
// copied if hard link is not possible).
func Install(pkgFile, destination string) error {
	mediaInfo, err := LoadMediaInfo(pkgFile)
	if err != nil {
		return err
	}
	switch mediaInfo.Type {
	case "application/x-tar":
		return extractTar(pkgFile, mediaInfo.Encoding, destination)
	case "application/zip":
		return extractZip(pkgFile, destination)
	case defaultType, "text/plain":
		if err := os.Link(pkgFile, destination); err != nil {
			// Can't create hard link (e.g., different filesystem).
			return copyFile(pkgFile, destination)
		}
		return nil
	default:
		// TODO(caprita): Instead of throwing an error, why not just
		// handle things with os.Link(pkgFile, destination) as the two
		// cases above?
		return verror.New(errBadMediaType, nil, mediaInfo.Type)
	}
}

// LoadMediaInfo returns the MediaInfo for the given package file.
func LoadMediaInfo(pkgFile string) (repository.MediaInfo, error) {
	jInfo, err := ioutil.ReadFile(MediaInfoFile(pkgFile))
	if err != nil {
		return repository.MediaInfo{}, err
	}
	var info repository.MediaInfo
	if err := json.Unmarshal(jInfo, &info); err != nil {
		return repository.MediaInfo{}, err
	}
	return info, nil
}

// SaveMediaInfo saves the media info for a package.
func SaveMediaInfo(pkgFile string, mediaInfo repository.MediaInfo) error {
	jInfo, err := json.Marshal(mediaInfo)
	if err != nil {
		return err
	}
	infoFile := MediaInfoFile(pkgFile)
	if err := ioutil.WriteFile(infoFile, jInfo, os.FileMode(0600)); err != nil {
		return err
	}
	return nil
}

// CreateZip creates a package from the files in the source directory. The
// created package is a Zip file.
func CreateZip(zipFile, sourceDir string) error {
	z, err := os.OpenFile(zipFile, os.O_CREATE|os.O_WRONLY, os.FileMode(0644))
	if err != nil {
		return err
	}
	defer z.Close()
	w := zip.NewWriter(z)
	if err := filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if sourceDir == path {
			return nil
		}
		fh, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		fh.Method = zip.Deflate
		fh.Name, _ = filepath.Rel(sourceDir, path)
		hdr, err := w.CreateHeader(fh)
		if err != nil {
			return err
		}
		if !info.IsDir() {
			content, err := ioutil.ReadFile(path)
			if err != nil {
				return err
			}
			if _, err = hdr.Write(content); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	if err := SaveMediaInfo(zipFile, repository.MediaInfo{Type: "application/zip"}); err != nil {
		return err
	}
	return nil
}

func extractZip(zipFile, installDir string) error {
	if err := os.Mkdir(installDir, os.FileMode(createDirMode)); err != nil {
		return verror.New(errMkDirFailed, nil, installDir, err)
	}
	zr, err := zip.OpenReader(zipFile)
	if err != nil {
		return err
	}
	for _, file := range zr.File {
		fi := file.FileInfo()
		name := filepath.Join(installDir, file.Name)
		if !strings.HasPrefix(name, installDir) {
			return verror.New(errFailedToExtract, nil, file.Name)
		}
		if fi.IsDir() {
			if err := os.MkdirAll(name, os.FileMode(createDirMode)); err != nil && !os.IsExist(err) {
				return err
			}
			continue
		}
		in, err := file.Open()
		if err != nil {
			return err
		}
		parentName := filepath.Dir(name)
		if err := os.MkdirAll(parentName, os.FileMode(createDirMode)); err != nil {
			return err
		}
		out, err := os.OpenFile(name, os.O_CREATE|os.O_WRONLY, os.FileMode(createFileMode))
		if err != nil {
			in.Close()
			return err
		}
		nbytes, err := io.Copy(out, in)
		in.Close()
		out.Close()
		if err != nil {
			return err
		}
		if nbytes != fi.Size() {
			return verror.New(errBadFileSize, nil, fi.Name(), nbytes, fi.Size())
		}
	}
	return nil
}

func extractTar(pkgFile string, encoding string, installDir string) error { //nolint:gocyclo
	if err := os.Mkdir(installDir, os.FileMode(createDirMode)); err != nil {
		return verror.New(errMkDirFailed, nil, installDir, err)
	}
	f, err := os.Open(pkgFile)
	if err != nil {
		return err
	}
	defer f.Close()

	var reader io.Reader
	switch encoding {
	case "":
		reader = f
	case "gzip":
		var err error
		if reader, err = gzip.NewReader(f); err != nil {
			return err
		}
	case "bzip2":
		reader = bzip2.NewReader(f)
	default:
		return verror.New(errBadEncoding, nil, encoding)
	}

	tr := tar.NewReader(reader)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		name := filepath.Join(installDir, hdr.Name)
		if !strings.HasPrefix(name, installDir) {
			return verror.New(errFailedToExtract, nil, hdr.Name)
		}
		// Regular file
		if hdr.Typeflag == tar.TypeReg {
			parentName := filepath.Dir(name)
			if err := os.MkdirAll(parentName, os.FileMode(createDirMode)); err != nil {
				return err
			}
			out, err := os.OpenFile(name, os.O_CREATE|os.O_WRONLY, os.FileMode(createFileMode))
			if err != nil {
				return err
			}
			nbytes, err := io.Copy(out, tr)
			out.Close()
			if err != nil {
				return err
			}
			if nbytes != hdr.Size {
				return verror.New(errBadFileSize, nil, hdr.Name, nbytes, hdr.Size)
			}
			continue
		}
		// Directory
		if hdr.Typeflag == tar.TypeDir {
			if err := os.MkdirAll(name, os.FileMode(createDirMode)); err != nil && !os.IsExist(err) {
				return err
			}
			continue
		}
		// Skip unsupported types
		// TODO(rthellend): Consider adding support for Symlink.
	}
}
