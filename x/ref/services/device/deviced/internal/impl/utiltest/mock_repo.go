// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package utiltest

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"reflect"
	"testing"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/security/access"
	"v.io/v23/services/application"
	"v.io/v23/services/binary"
	"v.io/v23/services/repository"
	"v.io/v23/verror"
)

const MockBinaryRepoName = "br"
const MockApplicationRepoName = "ar"

func StartMockRepos(t *testing.T, ctx *context.T) (*application.Envelope, func()) {
	envelope, appCleanup := StartApplicationRepository(ctx)
	binaryCleanup := StartBinaryRepository(ctx)

	return envelope, func() {
		binaryCleanup()
		appCleanup()
	}
}

// StartApplicationRepository sets up a server running the application
// repository.  It returns a pointer to the envelope that the repository returns
// to clients (so that it can be changed).  It also returns a cleanup function.
func StartApplicationRepository(ctx *context.T) (*application.Envelope, func()) {
	invoker := new(arInvoker)
	name := MockApplicationRepoName
	ctx, cancel := context.WithCancel(ctx)
	ctx, server, err := v23.WithNewServer(ctx, name, repository.ApplicationServer(invoker), security.AllowEveryone())
	if err != nil {
		ctx.Fatalf("NewServer(%v) failed: %v", name, err)
	}
	WaitForMount(ctx, ctx, name, server)
	return &invoker.envelope, func() {
		cancel()
		<-server.Closed()
	}
}

// arInvoker holds the state of an application repository invocation mock.  The
// mock returns the value of the wrapped envelope, which can be subsequently be
// changed at any time.  Client is responsible for synchronization if desired.
type arInvoker struct {
	envelope application.Envelope
}

// APPLICATION REPOSITORY INTERFACE IMPLEMENTATION
func (i *arInvoker) Match(ctx *context.T, _ rpc.ServerCall, profiles []string) (application.Envelope, error) {
	ctx.VI(1).Infof("Match()")
	if want := []string{"test-profile"}; !reflect.DeepEqual(profiles, want) {
		return application.Envelope{}, fmt.Errorf("Expected profiles %v, got %v", want, profiles)
	}
	return i.envelope, nil
}

func (i *arInvoker) GetPermissions(ctx *context.T, _ rpc.ServerCall) (perms access.Permissions, version string, err error) {
	return nil, "", nil
}

func (i *arInvoker) SetPermissions(ctx *context.T, _ rpc.ServerCall, perms access.Permissions, version string) error {
	return nil
}

func (i *arInvoker) TidyNow(_ *context.T, _ rpc.ServerCall) error {
	return nil
}

// brInvoker holds the state of a binary repository invocation mock.  It always
// serves the current running binary.
type brInvoker struct{}

// StartBinaryRepository sets up a server running the binary repository and
// returns a cleanup function.
func StartBinaryRepository(ctx *context.T) func() {
	name := MockBinaryRepoName
	ctx, cancel := context.WithCancel(ctx)
	ctx, server, err := v23.WithNewServer(ctx, name, repository.BinaryServer(new(brInvoker)), security.AllowEveryone())
	if err != nil {
		ctx.Fatalf("Serve(%q) failed: %v", name, err)
	}
	WaitForMount(ctx, ctx, name, server)
	return func() {
		cancel()
		<-server.Closed()
	}
}

// BINARY REPOSITORY INTERFACE IMPLEMENTATION

// TODO(toddw): Move the errors from dispatcher.go into a common location.
const pkgPath = "v.io/x/ref/services/device/deviced/internal/impl/utiltest"

var ErrOperationFailed = verror.Register(pkgPath+".OperationFailed", verror.NoRetry, "")

func (*brInvoker) Create(ctx *context.T, _ rpc.ServerCall, _ int32, _ repository.MediaInfo) error {
	ctx.VI(1).Infof("Create()")
	return nil
}

func (i *brInvoker) Delete(ctx *context.T, _ rpc.ServerCall) error {
	ctx.VI(1).Infof("Delete()")
	return nil
}

func mockBinaryBytesReader() (io.Reader, func(), error) {
	file, err := os.Open(os.Args[0])
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() {
		file.Close()
	}
	return file, cleanup, nil
}

func (i *brInvoker) Download(ctx *context.T, call repository.BinaryDownloadServerCall, _ int32) error {
	ctx.VI(1).Infof("Download()")
	file, cleanup, err := mockBinaryBytesReader()
	if err != nil {
		ctx.Errorf("Open() failed: %v", err)
		return verror.New(ErrOperationFailed, ctx)
	}
	defer cleanup()
	bufferLength := 4096
	buffer := make([]byte, bufferLength)
	sender := call.SendStream()
	for {
		n, err := file.Read(buffer)
		switch err {
		case io.EOF:
			return nil
		case nil:
			if err := sender.Send(buffer[:n]); err != nil {
				ctx.Errorf("Send() failed: %v", err)
				return verror.New(ErrOperationFailed, ctx)
			}
		default:
			ctx.Errorf("Read() failed: %v", err)
			return verror.New(ErrOperationFailed, ctx)
		}
	}
}

func (*brInvoker) DownloadUrl(ctx *context.T, _ rpc.ServerCall) (string, int64, error) {
	ctx.VI(1).Infof("DownloadUrl()")
	return "", 0, nil
}

func (*brInvoker) Stat(ctx *context.T, call rpc.ServerCall) ([]binary.PartInfo, repository.MediaInfo, error) {
	ctx.VI(1).Infof("Stat()")
	h := md5.New()
	bytes, err := ioutil.ReadFile(os.Args[0])
	if err != nil {
		return []binary.PartInfo{}, repository.MediaInfo{}, verror.New(ErrOperationFailed, ctx)
	}
	h.Write(bytes)
	part := binary.PartInfo{Checksum: hex.EncodeToString(h.Sum(nil)), Size: int64(len(bytes))}
	return []binary.PartInfo{part}, repository.MediaInfo{Type: "application/octet-stream"}, nil
}

func (i *brInvoker) Upload(ctx *context.T, _ repository.BinaryUploadServerCall, _ int32) error {
	ctx.VI(1).Infof("Upload()")
	return nil
}

func (i *brInvoker) GetPermissions(*context.T, rpc.ServerCall) (perms access.Permissions, version string, err error) {
	return nil, "", nil
}

func (i *brInvoker) SetPermissions(_ *context.T, _ rpc.ServerCall, perms access.Permissions, version string) error {
	return nil
}
