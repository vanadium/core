// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The implementation of the binary repository interface stores objects
// identified by object name suffixes using the local file system. Given an
// object name suffix, the implementation computes an MD5 hash of the suffix and
// generates the following path in the local filesystem:
// /<root-dir>/<dir_1>/.../<dir_n>/<hash>. The root directory and the directory
// depth are parameters of the implementation. <root-dir> also contains
// __acls/data and __acls/sig files storing the Permissions for the root level.
// The contents of the directory include the checksum and data for each of the
// individual parts of the binary, the name of the object and a directory
// containing the perms for this particular object:
//
// name
// acls/data
// acls/sig
// mediainfo
// name
// <part_1>/checksum
// <part_1>/data
// ...
// <part_n>/checksum
// <part_n>/data
//
// TODO(jsimsa): Add an "fsck" method that cleans up existing on-disk
// repository and provide a command-line flag that identifies whether
// fsck should run when new repository server process starts up.
package binarylib

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"v.io/v23/context"
	"v.io/v23/glob"
	"v.io/v23/naming"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/security/access"
	"v.io/v23/services/binary"
	"v.io/v23/services/repository"
	"v.io/v23/verror"
	"v.io/x/ref/services/internal/pathperms"
)

// binaryService implements the Binary server interface.
type binaryService struct {
	// path is the local filesystem path to the object identified by the
	// object name suffix.
	path string
	// state holds the state shared across different binary repository
	// invocations.
	state *state
	// suffix is the name of the binary object.
	suffix     string
	permsStore *pathperms.PathStore
}

const pkgPath = "v.io/x/ref/services/internal/binarylib"

var (
	ErrInProgress      = verror.Register(pkgPath+".errInProgress", verror.NoRetry, "{1:}{2:} identical upload already in progress{:_}")
	ErrInvalidParts    = verror.Register(pkgPath+".errInvalidParts", verror.NoRetry, "{1:}{2:} invalid number of binary parts{:_}")
	ErrInvalidPart     = verror.Register(pkgPath+".errInvalidPart", verror.NoRetry, "{1:}{2:} invalid binary part number{:_}")
	ErrOperationFailed = verror.Register(pkgPath+".errOperationFailed", verror.NoRetry, "{1:}{2:} operation failed{:_}")
	ErrNotAuthorized   = verror.Register(pkgPath+".errNotAuthorized", verror.NoRetry, "{1:}{2:} none of the client's blessings are valid {:_}")
	ErrInvalidSuffix   = verror.Register(pkgPath+".errInvalidSuffix", verror.NoRetry, "{1:}{2:} invalid suffix{:_}")
)

// TODO(jsimsa): When VDL supports composite literal constants, remove
// this definition.
var MissingPart = binary.PartInfo{
	Checksum: binary.MissingChecksum,
	Size:     binary.MissingSize,
}

// newBinaryService returns a new Binary service implementation.
func newBinaryService(state *state, suffix string, permsStore *pathperms.PathStore) *binaryService {
	return &binaryService{
		path:       state.dir(suffix),
		state:      state,
		suffix:     suffix,
		permsStore: permsStore,
	}
}

const BufferLength = 4096

func (i *binaryService) createFileTree(ctx *context.T, nparts int32, mediaInfo repository.MediaInfo) (string, error) {
	parent, dirPerm := filepath.Dir(i.path), os.FileMode(0700)
	if err := os.MkdirAll(parent, dirPerm); err != nil {
		ctx.Errorf("MkdirAll(%v, %v) failed: %v", parent, dirPerm, err)
		return "", verror.New(ErrOperationFailed, ctx)
	}
	prefix := "creating-"
	tmpDir, err := ioutil.TempDir(parent, prefix)
	if err != nil {
		ctx.Errorf("TempDir(%v, %v) failed: %v", parent, prefix, err)
		return "", verror.New(ErrOperationFailed, ctx)
	}
	nameFile, filePerm := filepath.Join(tmpDir, nameFileName), os.FileMode(0600)
	if err := ioutil.WriteFile(nameFile, []byte(i.suffix), filePerm); err != nil {
		ctx.Errorf("WriteFile(%q) failed: %v", nameFile, err)
		return "", verror.New(ErrOperationFailed, ctx)
	}
	infoFile := filepath.Join(tmpDir, mediaInfoFileName)
	jInfo, err := json.Marshal(mediaInfo)
	if err != nil {
		ctx.Errorf("json.Marshal(%v) failed: %v", mediaInfo, err)
		return "", verror.New(ErrOperationFailed, ctx)
	}
	if err := ioutil.WriteFile(infoFile, jInfo, filePerm); err != nil {
		ctx.Errorf("WriteFile(%q) failed: %v", infoFile, err)
		return "", verror.New(ErrOperationFailed, ctx)
	}
	for j := 0; j < int(nparts); j++ {
		partPath := generatePartPath(tmpDir, j)
		if err := os.MkdirAll(partPath, dirPerm); err != nil {
			ctx.Errorf("MkdirAll(%v, %v) failed: %v", partPath, dirPerm, err)
			if err := os.RemoveAll(tmpDir); err != nil {
				ctx.Errorf("RemoveAll(%v) failed: %v", tmpDir, err)
			}
			return "", verror.New(ErrOperationFailed, ctx)
		}
	}
	return tmpDir, nil
}

func (i *binaryService) deleteACLs(ctx *context.T) error {
	permsDir := permsPath(i.state.rootDir, i.suffix)
	if err := i.permsStore.Delete(permsDir); err != nil {
		return err
	}
	// HACK: we need to also clean up the parent directory (corresponding to
	// the suffix) that holds the "acls" directory for regular binary
	// objects.  See the implementation of permsPath.
	if base := filepath.Base(permsDir); base == "acls" {
		if err := os.Remove(filepath.Dir(permsDir)); err != nil {
			return err
		}
	}
	return nil
}

// setInitialPermissions sets the acls for the binary if they don't exist (if
// they do, it's a sign that the binary already exists, and then an error is
// returned).  Upon success, it returns a function that removes the permissions
// just set here (to be called if something fails downstream and we need to undo
// setting the initial permissions).
func (i *binaryService) setInitialPermissions(ctx *context.T, call rpc.ServerCall) (func(), error) {
	rb, _ := security.RemoteBlessingNames(ctx, call.Security())
	if len(rb) == 0 {
		// None of the client's blessings are valid.
		return nil, verror.New(ErrNotAuthorized, ctx)
	}
	permsDir := permsPath(i.state.rootDir, i.suffix)
	created, err := i.permsStore.SetIfAbsent(permsDir, pathperms.PermissionsForBlessings(rb))
	if err != nil {
		ctx.Errorf("permsStore.SetIfAbsent(%v, %v) failed: %v", permsDir, rb, err)
		return nil, verror.New(ErrOperationFailed, ctx)
	}
	if !created {
		return nil, verror.New(verror.ErrExist, ctx, i.suffix)
	}
	return func() {
		if err := i.deleteACLs(ctx); err != nil {
			ctx.Errorf("deleteACLs() failed: %v", err)
		}
	}, nil
}

func (i *binaryService) Create(ctx *context.T, call rpc.ServerCall, nparts int32, mediaInfo repository.MediaInfo) error {
	ctx.Infof("%v.Create(%v, %v)", i.suffix, nparts, mediaInfo)
	// Disallow creating binaries on the root of the server.  The
	// permissions on the root have special meaning (see
	// hierarchical_authorizer).
	if i.suffix == "" {
		return verror.New(ErrInvalidSuffix, ctx, "")
	}
	if nparts < 1 {
		return verror.New(ErrInvalidParts, ctx)
	}
	removePerms, err := i.setInitialPermissions(ctx, call)
	if err != nil {
		return err
	}
	tmpDir, err := i.createFileTree(ctx, nparts, mediaInfo)
	if err != nil {
		removePerms()
		return err
	}
	// Use os.Rename() to atomically create the binary directory
	// structure.
	if err := os.Rename(tmpDir, i.path); err != nil {
		ctx.Errorf("Rename(%v, %v) failed: %v", tmpDir, i.path, err)
		if err := os.RemoveAll(tmpDir); err != nil {
			ctx.Errorf("RemoveAll(%v) failed: %v", tmpDir, err)
		}
		removePerms()
		return verror.New(ErrOperationFailed, ctx, i.path)
	}
	return nil
}

func (i *binaryService) Delete(ctx *context.T, _ rpc.ServerCall) error {
	ctx.Infof("%v.Delete()", i.suffix)
	if _, err := os.Stat(i.path); err != nil {
		if os.IsNotExist(err) {
			return verror.New(verror.ErrNoExist, ctx, i.path)
		}
		ctx.Errorf("Stat(%v) failed: %v", i.path, err)
		return verror.New(ErrOperationFailed, ctx)
	}
	// Use os.Rename() to atomically remove the binary directory
	// structure.
	path := filepath.Join(filepath.Dir(i.path), "removing-"+filepath.Base(i.path))
	if err := os.Rename(i.path, path); err != nil {
		ctx.Errorf("Rename(%v, %v) failed: %v", i.path, path, err)
		return verror.New(ErrOperationFailed, ctx, i.path)
	}
	if err := os.RemoveAll(path); err != nil {
		ctx.Errorf("Remove(%v) failed: %v", path, err)
		return verror.New(ErrOperationFailed, ctx)
	}
	for {
		// Remove the binary and all directories on the path back to the
		// root directory that are left empty after the binary is removed.
		path = filepath.Dir(path)
		if i.state.rootDir == path {
			break
		}
		if err := os.Remove(path); err != nil {
			if err.(*os.PathError).Err.Error() == syscall.ENOTEMPTY.Error() {
				break
			}
			ctx.Errorf("Remove(%v) failed: %v", path, err)
			return verror.New(ErrOperationFailed, ctx)
		}
	}
	if err := i.deleteACLs(ctx); err != nil {
		ctx.Errorf("deleteACLs() failed: %v", err)
		return verror.New(ErrOperationFailed, ctx)
	}
	return nil
}

func (i *binaryService) Download(ctx *context.T, call repository.BinaryDownloadServerCall, part int32) error {
	ctx.Infof("%v.Download(%v)", i.suffix, part)
	path := i.generatePartPath(int(part))
	if err := checksumExists(ctx, path); err != nil {
		return err
	}
	dataPath := filepath.Join(path, dataFileName)
	file, err := os.Open(dataPath)
	if err != nil {
		ctx.Errorf("Open(%v) failed: %v", dataPath, err)
		return verror.New(ErrOperationFailed, ctx)
	}
	defer file.Close()
	buffer := make([]byte, BufferLength)
	sender := call.SendStream()
	for {
		n, err := file.Read(buffer)
		if err != nil && err != io.EOF {
			ctx.Errorf("Read() failed: %v", err)
			return verror.New(ErrOperationFailed, ctx)
		}
		if n == 0 {
			break
		}
		if err := sender.Send(buffer[:n]); err != nil {
			ctx.Errorf("Send() failed: %v", err)
			return verror.New(ErrOperationFailed, ctx)
		}
	}
	return nil
}

// TODO(jsimsa): Design and implement an access control mechanism for
// the URL-based downloads.
func (i *binaryService) DownloadUrl(ctx *context.T, _ rpc.ServerCall) (string, int64, error) {
	ctx.Infof("%v.DownloadUrl()", i.suffix)
	return i.state.rootURL + "/" + i.suffix, 0, nil
}

func (i *binaryService) Stat(ctx *context.T, _ rpc.ServerCall) ([]binary.PartInfo, repository.MediaInfo, error) {
	ctx.Infof("%v.Stat()", i.suffix)
	result := make([]binary.PartInfo, 0)
	parts, err := getParts(ctx, i.path)
	if err != nil {
		return []binary.PartInfo{}, repository.MediaInfo{}, err
	}
	for _, part := range parts {
		checksumFile := filepath.Join(part, checksumFileName)
		bytes, err := ioutil.ReadFile(checksumFile)
		if err != nil {
			if os.IsNotExist(err) {
				result = append(result, MissingPart)
				continue
			}
			ctx.Errorf("ReadFile(%v) failed: %v", checksumFile, err)
			return []binary.PartInfo{}, repository.MediaInfo{}, verror.New(ErrOperationFailed, ctx)
		}
		dataFile := filepath.Join(part, dataFileName)
		fi, err := os.Stat(dataFile)
		if err != nil {
			if os.IsNotExist(err) {
				result = append(result, MissingPart)
				continue
			}
			ctx.Errorf("Stat(%v) failed: %v", dataFile, err)
			return []binary.PartInfo{}, repository.MediaInfo{}, verror.New(ErrOperationFailed, ctx)
		}
		result = append(result, binary.PartInfo{Checksum: string(bytes), Size: fi.Size()})
	}
	infoFile := filepath.Join(i.path, mediaInfoFileName)
	jInfo, err := ioutil.ReadFile(infoFile)
	if err != nil {
		ctx.Errorf("ReadFile(%q) failed: %v", infoFile, err)
		return []binary.PartInfo{}, repository.MediaInfo{}, verror.New(ErrOperationFailed, ctx)
	}
	var mediaInfo repository.MediaInfo
	if err := json.Unmarshal(jInfo, &mediaInfo); err != nil {
		ctx.Errorf("json.Unmarshal(%v) failed: %v", jInfo, err)
		return []binary.PartInfo{}, repository.MediaInfo{}, verror.New(ErrOperationFailed, ctx)
	}
	return result, mediaInfo, nil
}

func (i *binaryService) Upload(ctx *context.T, call repository.BinaryUploadServerCall, part int32) error {
	ctx.Infof("%v.Upload(%v)", i.suffix, part)
	path, suffix := i.generatePartPath(int(part)), ""
	err := checksumExists(ctx, path)
	if err == nil {
		return verror.New(verror.ErrExist, ctx, path)
	} else if verror.ErrorID(err) != verror.ErrNoExist.ID {
		return err
	}
	// Use os.OpenFile() to resolve races.
	lockPath, flags, perm := filepath.Join(path, lockFileName), os.O_CREATE|os.O_WRONLY|os.O_EXCL, os.FileMode(0600)
	lockFile, err := os.OpenFile(lockPath, flags, perm)
	if err != nil {
		if os.IsExist(err) {
			return verror.New(ErrInProgress, ctx, path)
		}
		ctx.Errorf("OpenFile(%v, %v, %v) failed: %v", lockPath, flags, suffix, err)
		return verror.New(ErrOperationFailed, ctx)
	}
	defer os.Remove(lockFile.Name())
	defer lockFile.Close()
	file, err := ioutil.TempFile(path, suffix)
	if err != nil {
		ctx.Errorf("TempFile(%v, %v) failed: %v", path, suffix, err)
		return verror.New(ErrOperationFailed, ctx)
	}
	defer file.Close()
	h := md5.New()
	rStream := call.RecvStream()
	for rStream.Advance() {
		bytes := rStream.Value()
		if _, err := file.Write(bytes); err != nil {
			ctx.Errorf("Write() failed: %v", err)
			if err := os.Remove(file.Name()); err != nil {
				ctx.Errorf("Remove(%v) failed: %v", file.Name(), err)
			}
			return verror.New(ErrOperationFailed, ctx)
		}
		h.Write(bytes)
	}

	if err := rStream.Err(); err != nil {
		ctx.Errorf("Advance() failed: %v", err)
		if err := os.Remove(file.Name()); err != nil {
			ctx.Errorf("Remove(%v) failed: %v", file.Name(), err)
		}
		return verror.New(ErrOperationFailed, ctx)
	}

	hash := hex.EncodeToString(h.Sum(nil))
	checksumFile, perm := filepath.Join(path, checksumFileName), os.FileMode(0600)
	if err := ioutil.WriteFile(checksumFile, []byte(hash), perm); err != nil {
		ctx.Errorf("WriteFile(%v, %v, %v) failed: %v", checksumFile, hash, perm, err)
		if err := os.Remove(file.Name()); err != nil {
			ctx.Errorf("Remove(%v) failed: %v", file.Name(), err)
		}
		return verror.New(ErrOperationFailed, ctx)
	}
	dataFile := filepath.Join(path, dataFileName)
	if err := os.Rename(file.Name(), dataFile); err != nil {
		ctx.Errorf("Rename(%v, %v) failed: %v", file.Name(), dataFile, err)
		if err := os.Remove(file.Name()); err != nil {
			ctx.Errorf("Remove(%v) failed: %v", file.Name(), err)
		}
		return verror.New(ErrOperationFailed, ctx)
	}
	return nil
}

func (i *binaryService) GlobChildren__(ctx *context.T, call rpc.GlobChildrenServerCall, m *glob.Element) error {
	elems := strings.Split(i.suffix, "/")
	if len(elems) == 1 && elems[0] == "" {
		elems = nil
	}
	n := i.createObjectNameTree().find(elems, false)
	if n == nil {
		return verror.New(ErrOperationFailed, ctx)
	}
	for k, _ := range n.children {
		if m.Match(k) {
			call.SendStream().Send(naming.GlobChildrenReplyName{Value: k})
		}
	}
	return nil
}

func (i *binaryService) GetPermissions(ctx *context.T, call rpc.ServerCall) (perms access.Permissions, version string, err error) {
	perms, version, err = i.permsStore.Get(permsPath(i.state.rootDir, i.suffix))
	if os.IsNotExist(err) {
		// No Permissions file found which implies a nil authorizer. This results in
		// default authorization.
		return pathperms.NilAuthPermissions(ctx, call.Security()), "", nil
	}
	return perms, version, err
}

func (i *binaryService) SetPermissions(_ *context.T, _ rpc.ServerCall, perms access.Permissions, version string) error {
	return i.permsStore.Set(permsPath(i.state.rootDir, i.suffix), perms, version)
}
