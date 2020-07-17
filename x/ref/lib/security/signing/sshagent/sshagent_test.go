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

// It turns out that ssh-add behaves differently on different systems with
// respect to how keys are named in the agent. On macos, the -C comment
// field is respected. However on linux, or at least on circleci's go
// images, the -C comment is not respected for ecdsa keys and instead
// the filename used with ssh-add is used as the comment. In this test
// care is taken to ensure that the -C comments and filenames used
// as the same.

func sshkeygen(dir, filename string, args ...string) error {
	filepath := filepath.Join(dir, filename)
	args = append(args, "-f", filepath, "-N", "")
	cmd := exec.Command("ssh-keygen", args...)
	fmt.Println(strings.Join(cmd.Args, " "))
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Failed: %v: %v\n", strings.Join(cmd.Args, " "), err)
		fmt.Println(string(output))
		return err
	}
	buf, _ := ioutil.ReadFile(filepath)
	fmt.Println(string(buf))
	buf, _ = ioutil.ReadFile(filepath + ".pub")
	fmt.Println(string(buf))
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

func startAndConfigureAgent() (func(), error) {
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
	// Configure the sshagent package to use this agent and not the
	// user's existing one.
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
		"rsa",
		"ecdsa-256",
		"ecdsa-384",
		"ecdsa-521",
		"ed25519")
	cmd.Dir = tmpdir
	cmd.Env = []string{"SSH_AUTH_SOCK=" + addr}
	if output, err := cmd.CombinedOutput(); err != nil {
		return cleanup, fmt.Errorf("failed to add ssh keys: %v: %s", err, output)
	}
	cmd = exec.Command("ssh-add", "-l")
	cmd.Env = []string{"SSH_AUTH_SOCK=" + addr}
	output, _ = cmd.CombinedOutput()
	fmt.Printf("%s\n", output)
	return cleanup, nil
}

func TestMain(m *testing.M) {
	cleanup, err := startAndConfigureAgent()
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

func testAgentSigningVanadiumVerification(ctx context.Context, t *testing.T, passphrase []byte) {
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
		signer, err := service.Signer(ctx, keyComment, passphrase)
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

func TestAgentSigningVanadiumVerification(t *testing.T) {
	ctx := context.Background()
	testAgentSigningVanadiumVerification(ctx, t, nil)
}

func TestAgentSigningVanadiumVerificationPassphrase(t *testing.T) {
	ctx := context.Background()
	passphrase := []byte("something")
	service := sshagent.NewSigningService()
	agent := service.(*sshagent.Client)
	if err := agent.Lock(passphrase); err != nil {
		t.Fatalf("agent.Lock: %v", err)
	}
	_, err := service.Signer(ctx, "ed25519", nil)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("service.Signer: should have failed with a key not found error: %v", err)
	}
	testAgentSigningVanadiumVerification(ctx, t, passphrase)

	// make sure agent is still locked.
	_, err = service.Signer(ctx, "ed25519", nil)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("service.Signer: should have failed with a key not found error: %v", err)
	}

}
