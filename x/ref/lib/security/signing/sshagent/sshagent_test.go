package sshagent_test

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"v.io/x/ref/lib/security/signing/sshagent"
)

func sshkeygen(dir, filename string, args ...string) error {
	args = append(args, "-f", filepath.Join(dir, filename), "-N", "")
	cmd := exec.Command("ssh-keygen", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(string(output))
		return err
	}
	return nil
}

func generateKeys(dir string) error {
	for _, err := range []error{
		sshkeygen(dir, "rsa", "-t", "rsa", "-C", "rsa"),
		sshkeygen(dir, "ecdsa-256", "-t", "ecdsa", "-b", "256", "-C", "ecdsa-256"),
		sshkeygen(dir, "ecdsa-384", "-t", "ecdsa", "-b", "384", "-C", "ecdsa-384"),
		sshkeygen(dir, "ecdsa-521", "-t", "ecdsa", "-b", "521", "-C", "ecdsa-521"),
		sshkeygen(dir, "ed25519", "-t", "ed25519", "-C", "ed25519"),
	} {
		if err != nil {
			return err
		}
	}
	return nil
}

func startAgent() (func(), error) {
	tmpdir, err := ioutil.TempDir("", "ssh-keygen")
	if err != nil {
		return nil, err
	}
	if err := generateKeys(tmpdir); err != nil {
		return nil, err
	}

	cmd := exec.Command("ssh-agent")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(output), "\n")
	first := lines[0]
	addr := strings.TrimPrefix(first, "SSH_AUTH_SOCK=")
	addr = strings.TrimSuffix(addr, "; export SSH_AUTH_SOCK;")
	sshagent.SetAgentAddress(func() string {
		return addr
	})
	second := lines[1]
	pidstr := strings.TrimPrefix(second, "SSH_AGENT_PID=")
	pidstr = strings.TrimSuffix(pidstr, "; export SSH_AGENT_PID;")
	pid, err := strconv.ParseInt(pidstr, 10, 64)
	if err != nil {
		return func() {}, fmt.Errorf("failed to parse pid from %v", second)
	}

	cleanup := func() {
		syscall.Kill(int(pid), syscall.SIGTERM)
		if testing.Verbose() {
			fmt.Println(string(output))
			fmt.Printf("killing: %v\n", int(pid))
		}
	}

	cmd = exec.Command("ssh-add",
		filepath.Join(tmpdir, "rsa"),
		filepath.Join(tmpdir, "ecdsa-256"),
		filepath.Join(tmpdir, "ecdsa-384"),
		filepath.Join(tmpdir, "ecdsa-521"),
		filepath.Join(tmpdir, "ed25519"))
	cmd.Env = []string{"SSH_AUTH_SOCK=" + addr}
	if output, err := cmd.CombinedOutput(); err != nil {
		return cleanup, fmt.Errorf("failed to add ssh keys: %v: %s", err, output)
	}
	return cleanup, nil
}

func TestMain(m *testing.M) {
	cleanup, err := startAgent()
	if err != nil {
		flag.Parse()
		cleanup()
		fmt.Fprintf(os.Stderr, "failed to start/configure agent: %v\n", err)
		os.Exit(1)
	}
	code := m.Run()
	cleanup()
	os.Exit(code)
}

func TestAgentSigningVanadiumVerification(t *testing.T) {
	ctx := context.Background()
	service := sshagent.NewSigningService()
	randSource := rand.New(rand.NewSource(time.Now().UnixNano()))
	for _, keyComment := range []string{
		"ecdsa-256",
		"ecdsa-384",
		"ecdsa-521",
		"ed25519",
	} {
		data := make([]byte, 4096)
		_, err := randSource.Read(data)
		if err != nil {
			t.Fatalf("rand: %v", err)
		}
		signer, err := service.Signer(ctx, keyComment, nil)
		if err != nil {
			t.Fatalf("service.Signer: %v", err)
		}
		sig, err := signer.Sign([]byte("testing"), data)
		if err != nil {
			t.Fatalf("signer.Sign: %v", err)
		}
		// Verify using Vanadium code.
		publicKey := signer.PublicKey()
		if !sig.Verify(publicKey, data) {
			t.Errorf("failed to verify signature for %v", keyComment)
		}
		data[1]++
		if sig.Verify(publicKey, data) {
			t.Errorf("failed to detect changed message for %v", keyComment)
		}
	}
	defer service.Close(ctx)
}

// test agent passphrase locking
// test agent errors, run sshagent mock server, or just kill
// existing one?
