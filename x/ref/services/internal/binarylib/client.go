// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package binarylib

// TODO(jsimsa): Implement parallel download and upload.

import (
	"bytes"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/security"
	"v.io/v23/services/binary"
	"v.io/v23/services/repository"
	"v.io/v23/verror"
	"v.io/x/ref/services/internal/packages"
)

var (
	errOperationFailed = verror.Register(pkgPath+".errOperationFailed", verror.NoRetry, "{1:}{2:} operation failed{:_}")
)

const (
	nAttempts   = 2
	partSize    = 1 << 22
	subpartSize = 1 << 12
)

func Delete(ctx *context.T, name string) error {
	if err := repository.BinaryClient(name).Delete(ctx); err != nil {
		ctx.Errorf("Delete() failed: %v", err)
		return err
	}
	return nil
}

type indexedPart struct {
	part   binary.PartInfo
	index  int
	offset int64
}

func downloadPartAttempt(ctx *context.T, w io.WriteSeeker, client repository.BinaryClientStub, ip *indexedPart) bool {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if _, err := w.Seek(ip.offset, 0); err != nil {
		ctx.Errorf("Seek(%v, 0) failed: %v", ip.offset, err)
		return false
	}
	stream, err := client.Download(ctx, int32(ip.index))
	if err != nil {
		ctx.Errorf("Download(%v) failed: %v", ip.index, err)
		return false
	}
	h, nreceived := md5.New(), 0
	rStream := stream.RecvStream()
	for rStream.Advance() {
		bytes := rStream.Value()
		if _, err := w.Write(bytes); err != nil {
			ctx.Errorf("Write() failed: %v", err)
			return false
		}
		h.Write(bytes) //nolint:errcheck
		nreceived += len(bytes)
	}

	if err := rStream.Err(); err != nil {
		ctx.Errorf("Advance() failed: %v", err)
		return false
	}
	if err := stream.Finish(); err != nil {
		ctx.Errorf("Finish() failed: %v", err)
		return false
	}
	if expected, got := ip.part.Checksum, hex.EncodeToString(h.Sum(nil)); expected != got {
		ctx.Errorf("Unexpected checksum: expected %v, got %v", expected, got)
		return false
	}
	if expected, got := ip.part.Size, int64(nreceived); expected != got {
		ctx.Errorf("Unexpected size: expected %v, got %v", expected, got)
		return false
	}
	return true
}

func downloadPart(ctx *context.T, w io.WriteSeeker, client repository.BinaryClientStub, ip *indexedPart) bool {
	for i := 0; i < nAttempts; i++ {
		if downloadPartAttempt(ctx, w, client, ip) {
			return true
		}
	}
	return false
}

func Stat(ctx *context.T, name string) (repository.MediaInfo, error) {
	client := repository.BinaryClient(name)
	_, mediaInfo, err := client.Stat(ctx)
	if err != nil {
		return repository.MediaInfo{}, err
	}
	return mediaInfo, nil
}

func download(ctx *context.T, w io.WriteSeeker, von string) (repository.MediaInfo, error) {
	client := repository.BinaryClient(von)
	parts, mediaInfo, err := client.Stat(ctx)
	if err != nil {
		ctx.Errorf("Stat() failed: %v", err)
		return repository.MediaInfo{}, err
	}
	for _, part := range parts {
		if part.Checksum == binary.MissingChecksum {
			return repository.MediaInfo{}, verror.New(verror.ErrNoExist, ctx)
		}
	}
	offset := int64(0)
	for i, part := range parts {
		ip := &indexedPart{part, i, offset}
		if !downloadPart(ctx, w, client, ip) {
			return repository.MediaInfo{}, verror.New(errOperationFailed, ctx)
		}
		offset += part.Size
	}
	return mediaInfo, nil
}

func Download(ctx *context.T, von string) ([]byte, repository.MediaInfo, error) {
	dir, prefix := "", ""
	file, err := ioutil.TempFile(dir, prefix)
	if err != nil {
		ctx.Errorf("TempFile(%v, %v) failed: %v", dir, prefix, err)
		return nil, repository.MediaInfo{}, verror.New(errOperationFailed, ctx)
	}
	defer os.Remove(file.Name())
	defer file.Close()
	mediaInfo, err := download(ctx, file, von)
	if err != nil {
		return nil, repository.MediaInfo{}, verror.New(errOperationFailed, ctx)
	}
	bytes, err := ioutil.ReadFile(file.Name())
	if err != nil {
		ctx.Errorf("ReadFile(%v) failed: %v", file.Name(), err)
		return nil, repository.MediaInfo{}, verror.New(errOperationFailed, ctx)
	}
	return bytes, mediaInfo, nil
}

func DownloadToFile(ctx *context.T, von, path string) error {
	dir := filepath.Dir(path)
	prefix := fmt.Sprintf(".download.%s.", filepath.Base(path))
	file, err := ioutil.TempFile(dir, prefix)
	if err != nil {
		ctx.Errorf("TempFile(%v, %v) failed: %v", dir, prefix, err)
		return verror.New(errOperationFailed, ctx)
	}
	defer file.Close()
	mediaInfo, err := download(ctx, file, von)
	if err != nil {
		if err := os.Remove(file.Name()); err != nil {
			ctx.Errorf("Remove(%v) failed: %v", file.Name(), err)
		}
		return verror.New(errOperationFailed, ctx)
	}
	perm := os.FileMode(0600)
	if err := file.Chmod(perm); err != nil {
		ctx.Errorf("Chmod(%v) failed: %v", perm, err)
		if err := os.Remove(file.Name()); err != nil {
			ctx.Errorf("Remove(%v) failed: %v", file.Name(), err)
		}
		return verror.New(errOperationFailed, ctx)
	}
	if err := os.Rename(file.Name(), path); err != nil {
		ctx.Errorf("Rename(%v, %v) failed: %v", file.Name(), path, err)
		if err := os.Remove(file.Name()); err != nil {
			ctx.Errorf("Remove(%v) failed: %v", file.Name(), err)
		}
		return verror.New(errOperationFailed, ctx)
	}
	if err := packages.SaveMediaInfo(path, mediaInfo); err != nil {
		ctx.Errorf("packages.SaveMediaInfo(%v, %v) failed: %v", path, mediaInfo, err)
		if err := os.Remove(path); err != nil {
			ctx.Errorf("Remove(%v) failed: %v", path, err)
		}
		return verror.New(errOperationFailed, ctx)
	}
	return nil
}

func DownloadUrl(ctx *context.T, von string) (string, int64, error) {
	url, ttl, err := repository.BinaryClient(von).DownloadUrl(ctx)
	if err != nil {
		ctx.Errorf("DownloadUrl() failed: %v", err)
		return "", 0, err
	}
	return url, ttl, nil
}

func uploadPartAttempt(ctx *context.T, h hash.Hash, r io.ReadSeeker, client repository.BinaryClientStub, part int, size int64) (bool, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	offset := int64(part * partSize)
	if _, err := r.Seek(offset, 0); err != nil {
		ctx.Errorf("Seek(%v, 0) failed: %v", offset, err)
		return false, nil
	}
	stream, err := client.Upload(ctx, int32(part))
	if err != nil {
		ctx.Errorf("Upload(%v) failed: %v", part, err)
		return false, nil
	}
	bufferSize := partSize
	if remaining := size - offset; remaining < int64(bufferSize) {
		bufferSize = int(remaining)
	}
	buffer := make([]byte, bufferSize)

	nread := 0
	for nread < len(buffer) {
		n, err := r.Read(buffer[nread:])
		nread += n
		if err != nil && (err != io.EOF || nread < len(buffer)) {
			ctx.Errorf("Read() failed: %v", err)
			return false, nil
		}
	}
	sender := stream.SendStream()
	for from := 0; from < len(buffer); from += subpartSize {
		to := from + subpartSize
		if to > len(buffer) {
			to = len(buffer)
		}
		if err := sender.Send(buffer[from:to]); err != nil {
			ctx.Errorf("Send() failed: %v", err)
			return false, nil
		}
	}
	// TODO(gauthamt): To detect corruption, the upload checksum needs
	// to be computed here rather than on the binary server.
	if err := sender.Close(); err != nil {
		ctx.Errorf("Close() failed: %v", err)
		parts, _, statErr := client.Stat(ctx)
		if statErr != nil {
			ctx.Errorf("Stat() failed: %v", statErr)
			if deleteErr := client.Delete(ctx); err != nil {
				ctx.Errorf("Delete() failed: %v", deleteErr)
			}
			return false, err
		}
		if parts[part].Checksum == binary.MissingChecksum {
			return false, nil
		}
	}
	if err := stream.Finish(); err != nil {
		ctx.Errorf("Finish() failed: %v", err)
		parts, _, statErr := client.Stat(ctx)
		if statErr != nil {
			ctx.Errorf("Stat() failed: %v", statErr)
			if deleteErr := client.Delete(ctx); err != nil {
				ctx.Errorf("Delete() failed: %v", deleteErr)
			}
			return false, err
		}
		if parts[part].Checksum == binary.MissingChecksum {
			return false, nil
		}
	}
	h.Write(buffer) //nolint:errcheck
	return true, nil
}

func uploadPart(ctx *context.T, h hash.Hash, r io.ReadSeeker, client repository.BinaryClientStub, part int, size int64) error {
	for i := 0; i < nAttempts; i++ {
		if success, err := uploadPartAttempt(ctx, h, r, client, part, size); success || err != nil {
			return err
		}
	}
	return verror.New(errOperationFailed, ctx)
}

func upload(ctx *context.T, r io.ReadSeeker, mediaInfo repository.MediaInfo, von string) (*security.Signature, error) {
	client := repository.BinaryClient(von)
	offset, whence := int64(0), 2
	size, err := r.Seek(offset, whence)
	if err != nil {
		ctx.Errorf("Seek(%v, %v) failed: %v", offset, whence, err)
		return nil, verror.New(errOperationFailed, ctx)
	}
	nparts := (size-1)/partSize + 1
	if err := client.Create(ctx, int32(nparts), mediaInfo); err != nil {
		ctx.Errorf("Create() failed: %v", err)
		return nil, err
	}
	h := sha256.New()
	for i := 0; int64(i) < nparts; i++ {
		if err := uploadPart(ctx, h, r, client, i, size); err != nil {
			return nil, err
		}
	}
	return signHash(ctx, h)
}

func signHash(ctx *context.T, h hash.Hash) (*security.Signature, error) {
	hash := h.Sum(nil)
	sig, err := v23.GetPrincipal(ctx).Sign(hash[:])
	if err != nil {
		ctx.Errorf("Sign() of hash failed:%v", err)
		return nil, err
	}
	return &sig, nil
}

func Upload(ctx *context.T, von string, data []byte, mediaInfo repository.MediaInfo) (*security.Signature, error) {
	buffer := bytes.NewReader(data)
	return upload(ctx, buffer, mediaInfo, von)
}

func Sign(ctx *context.T, in io.Reader) (*security.Signature, error) {
	out := sha256.New()
	if _, err := io.Copy(out, in); err != nil {
		return nil, err
	}
	return signHash(ctx, out)
}

func UploadFromFile(ctx *context.T, von, path string) (*security.Signature, error) {
	file, err := os.Open(path)
	if err != nil {
		ctx.Errorf("Open(%v) failed: %v", path, err)
		return nil, verror.New(errOperationFailed, ctx)
	}
	defer file.Close()
	mediaInfo, err := packages.LoadMediaInfo(path)
	if err != nil {
		mediaInfo = packages.MediaInfoForFileName(path)
	}
	return upload(ctx, file, mediaInfo, von)
}

func UploadFromDir(ctx *context.T, von, sourceDir string) (*security.Signature, error) {
	dir, err := ioutil.TempDir("", "create-package-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	zipfile := filepath.Join(dir, "file.zip")
	if err := packages.CreateZip(zipfile, sourceDir); err != nil {
		return nil, err
	}
	return UploadFromFile(ctx, von, zipfile)
}
