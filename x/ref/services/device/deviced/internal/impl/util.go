// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package impl

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"time"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/security"
	"v.io/v23/services/application"
	"v.io/v23/services/repository"
	"v.io/v23/verror"
	"v.io/x/ref/services/device/internal/config"
	"v.io/x/ref/services/device/internal/errors"
	"v.io/x/ref/services/internal/binarylib"
)

// TODO(caprita): Set these timeout in a more principled manner.
const (
	childReadyTimeout     = 40 * time.Second
	childWaitTimeout      = 40 * time.Second
	rpcContextTimeout     = time.Minute
	rpcContextLongTimeout = 5 * time.Minute
)

func verifySignature(data []byte, publisher security.Blessings, sig security.Signature) error {
	if !publisher.IsZero() {
		h := sha256.Sum256(data)
		if !sig.Verify(publisher.PublicKey(), h[:]) {
			return verror.New(errors.ErrOperationFailed, nil)
		}
	}
	return nil
}

func downloadBinary(ctx *context.T, publisher security.Blessings, bin *application.SignedFile, workspace, fileName string) error {
	// TODO(gauthamt): Reduce the number of passes we make over the binary/package
	// data to verify its checksum and signature.
	data, _, err := binarylib.Download(ctx, bin.File)
	if err != nil {
		return verror.New(errors.ErrOperationFailed, ctx, fmt.Sprintf("Download(%v) failed: %v", bin.File, err))
	}
	if err := verifySignature(data, publisher, bin.Signature); err != nil {
		return verror.New(errors.ErrOperationFailed, ctx, fmt.Sprintf("Publisher binary(%v) signature verification failed", bin.File))
	}
	path, perm := filepath.Join(workspace, fileName), os.FileMode(0755)
	if err := ioutil.WriteFile(path, data, perm); err != nil {
		return verror.New(errors.ErrOperationFailed, ctx, fmt.Sprintf("WriteFile(%v, %v) failed: %v", path, perm, err))
	}
	return nil
}

// TODO(caprita): share code between downloadBinary and downloadPackages.
func downloadPackages(ctx *context.T, publisher security.Blessings, packages application.Packages, pkgDir string) error {
	for localPkg, pkgName := range packages {
		if localPkg == "" || localPkg[0] == '.' || strings.Contains(localPkg, string(filepath.Separator)) {
			return verror.New(errors.ErrOperationFailed, ctx, fmt.Sprintf("invalid local package name: %q", localPkg))
		}
		path := filepath.Join(pkgDir, localPkg)
		if err := binarylib.DownloadToFile(ctx, pkgName.File, path); err != nil {
			return verror.New(errors.ErrOperationFailed, ctx, fmt.Sprintf("DownloadToFile(%q, %q) failed: %v", pkgName, path, err))
		}
		data, err := ioutil.ReadFile(path)
		if err != nil {
			return verror.New(errors.ErrOperationFailed, ctx, fmt.Sprintf("ReadPackage(%v) failed: %v", path, err))
		}
		// If a nonempty signature is present, verify it. (i.e., we accept unsigned packages.)
		if !reflect.DeepEqual(pkgName.Signature, security.Signature{}) {
			if err := verifySignature(data, publisher, pkgName.Signature); err != nil {
				return verror.New(errors.ErrOperationFailed, ctx, fmt.Sprintf("Publisher package(%v:%v) signature verification failed", localPkg, pkgName))
			}
		}
	}
	return nil
}

func fetchEnvelope(ctx *context.T, origin string) (*application.Envelope, error) {
	stub := repository.ApplicationClient(origin)
	profilesSet, err := Describe()
	if err != nil {
		return nil, verror.New(errors.ErrOperationFailed, ctx, fmt.Sprintf("Failed to obtain profile labels: %v", err))
	}
	var profiles []string
	for label := range profilesSet.Profiles {
		profiles = append(profiles, label)
	}
	envelope, err := stub.Match(ctx, profiles)
	if err != nil {
		return nil, verror.New(errors.ErrOperationFailed, ctx, fmt.Sprintf("Match(%v) failed: %v", profiles, err))
	}
	// If a publisher blessing is present, it must be from a publisher we recognize. If not,
	// reject the envelope. Note that unsigned envelopes are accepted by this check.
	// TODO: Implment a real ACL check based on publisher
	names, rejected := publisherBlessingNames(ctx, envelope)
	if len(names) == 0 && len(rejected) > 0 {
		return nil, verror.New(errors.ErrOperationFailed, ctx, fmt.Sprintf("publisher %v in envelope %v was not recognized", rejected, envelope.Title))
	}
	return &envelope, nil
}

func publisherBlessingNames(ctx *context.T, env application.Envelope) ([]string, []security.RejectedBlessing) {
	p := v23.GetPrincipal(ctx)
	b, _ := p.BlessingStore().Default()
	call := security.NewCall(&security.CallParams{
		RemoteBlessings: env.Publisher,
		LocalBlessings:  b,
		LocalPrincipal:  p,
		Timestamp:       time.Now(),
	})
	names, rejected := security.RemoteBlessingNames(ctx, call)
	if len(rejected) > 0 {
		ctx.Infof("For envelope %v, rejected publisher blessings: %v", env.Title, rejected)
	}
	ctx.VI(2).Infof("accepted publisher blessings: %v", names)
	return names, rejected
}

// LinkSelf creates a link to the current binary.
func LinkSelf(workspace, fileName string) error {
	path := filepath.Join(workspace, fileName)
	self := os.Args[0]
	if err := os.Link(self, path); err != nil {
		return verror.New(errors.ErrOperationFailed, nil, fmt.Sprintf("Link(%v, %v) failed: %v", self, path, err))
	}
	return nil
}

func generateVersionDirName() string {
	// TODO(caprita): Use generateID instead.
	return time.Now().Format(time.RFC3339Nano)
}

func UpdateLink(target, link string) error {
	newLink := link + ".new"
	fi, err := os.Lstat(newLink)
	if err == nil {
		if err := os.Remove(fi.Name()); err != nil {
			return verror.New(errors.ErrOperationFailed, nil, fmt.Sprintf("Remove(%v) failed: %v", fi.Name(), err))
		}
	}
	if err := os.Symlink(target, newLink); err != nil {
		return verror.New(errors.ErrOperationFailed, nil, fmt.Sprintf("Symlink(%v, %v) failed: %v", target, newLink, err))
	}
	if err := os.Rename(newLink, link); err != nil {
		return verror.New(errors.ErrOperationFailed, nil, fmt.Sprintf("Rename(%v, %v) failed: %v", newLink, link, err))
	}
	return nil
}

func BaseCleanupDir(ctx *context.T, path, helper string) {
	if helper != "" {
		out, err := exec.Command(helper, "--rm", path).CombinedOutput()
		if err != nil {
			ctx.Errorf("exec.Command(%s %s %s).CombinedOutput() failed: %v", helper, "--rm", path, err)
			return
		}
		if len(out) != 0 {
			ctx.Errorf("exec.Command(%s %s %s).CombinedOutput() generated output: %v", helper, "--rm", path, string(out))
		}
	} else {
		if err := os.RemoveAll(path); err != nil {
			ctx.Errorf("RemoveAll(%v) failed: %v", path, err)
		}
	}
}

func PermsDir(c *config.State) string {
	return filepath.Join(c.Root, "device-manager", "device-data", "acls")
}

// CleanupDir is defined like this so we can override its implementation for
// tests. CleanupDir will use the helper to delete application state possibly
// owned by different accounts if helper is provided.
var CleanupDir = BaseCleanupDir

// VanadiumEnvironment returns only the environment variables that are specific
// to the Vanadium system.
func VanadiumEnvironment(env []string) []string {
	return filterEnvironment(env, allowedVarsRE, deniedVarsRE)
}

var allowedVarsRE = regexp.MustCompile("^(V23_.*|GOSH_.*|PAUSE_BEFORE_STOP|TMPDIR|PATH)$")

var deniedVarsRE = regexp.MustCompile("^(V23_EXEC_VERSION|V23_EXEC_CONFIG)$")

// filterEnvironment returns only the environment variables, specified by
// the env parameter, whose names match the supplied regexp.
func filterEnvironment(env []string, allow, deny *regexp.Regexp) []string {
	var ret []string
	for _, e := range env {
		if eqIdx := strings.Index(e, "="); eqIdx > 0 {
			key := e[:eqIdx]
			if deny.MatchString(key) {
				continue
			}
			if allow.MatchString(key) {
				ret = append(ret, e)
			}
		}
	}
	return ret
}

// generateRandomString returns a cryptographically-strong random string.
func generateRandomString() (string, error) {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// generateAgentSockDir returns the name of a newly created directory where to
// create an agent socket.
func generateAgentSockDir(rootDir string) (string, error) {
	randomPattern, err := generateRandomString()
	if err != nil {
		return "", err
	}
	// We keep the socket files close to the root dir of the device
	// manager installation to ensure that the socket file path is
	// shorter than 108 characters (a requirement on Linux).
	sockDir := filepath.Join(rootDir, "socks", randomPattern)
	// TODO(caprita): For multi-user mode, we should chown the
	// socket dir to the app user, and set up a unix group to permit
	// access to the socket dir to the agent and device manager.
	// For now, 'security' hinges on the fact that the name of the
	// socket dir is unknown to everyone except the device manager,
	// the agent, and the app.
	if err := os.MkdirAll(sockDir, 0711); err != nil {
		return "", fmt.Errorf("MkdirAll(%q) failed: %v", sockDir, err)
	}
	return sockDir, nil
}
