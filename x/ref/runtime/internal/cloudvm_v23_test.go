package internal_test

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/namespace"
	"v.io/v23/naming"
	"v.io/v23/verror"
	"v.io/x/ref/runtime/factories/library"
	"v.io/x/ref/test/v23test"
)

func init() {
	library.AllowMultipleInitializations = true
	library.CloudVM = true
}

type serverInfo struct {
	cmd *v23test.Cmd
	log *bytes.Buffer
}

func startServer(t *testing.T, sh *v23test.Shell, args ...string) *serverInfo {
	var (
		binary = v23test.BuildGoPkg(sh, "v.io/x/ref/runtime/internal/cmd/cloudvm")
		log    = &bytes.Buffer{}
	)
	cmd := sh.Cmd(binary, args...)
	cmd.AddStdoutWriter(log)
	cmd.AddStderrWriter(log)
	cmd.Start()
	return &serverInfo{
		cmd: cmd,
		log: log,
	}
}

func TestV23Virtualized(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	sh.StartRootMountTable()

	ns := v23.GetNamespace(sh.Ctx)
	ns.CacheCtl(naming.DisableCache(true))

	fmt.Printf("running server\n")
	si := startServer(t, sh,
		"--name=dns",
		"--v23.virtualized.dns.public-name=loadbalancer.com")

	var err error
	assert := func(msg string, logs ...*bytes.Buffer) {
		if err != nil {
			for i, log := range logs {
				t.Log("-------------")
				t.Logf("log from: %s:%v: %v", msg, i, log.String())
				t.Log("-------------")
			}
			t.Fatalf("%s: %s", msg, err)
		}
	}

	fmt.Printf("waiting for server\n")
	servers, err := waitForMountedServers(t, sh.Ctx, ns, "dns")
	assert("dns-name", si.log)
	for i, s := range servers {
		fmt.Printf("%v: %v\n", i, s.Server)
	}
	t.Fail()
}

func waitForMountedServers(t *testing.T, ctx *context.T, ns namespace.T, name string) ([]naming.MountedServer, error) {
	start := time.Now()
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	for {
		resolved, err := ns.Resolve(ctx, name)
		if err != nil {
			if verror.ErrorID(err) == naming.ErrNoSuchName.ID {
				time.Sleep(time.Millisecond * 200)
				continue
			}
			return nil, err
		}
		fmt.Printf("X: %v\n", resolved.Servers)
		if len(resolved.Servers) > 0 {
			return resolved.Servers, nil
		}
		if time.Since(start) > time.Second*5 {
			break
		}
		time.Sleep(time.Millisecond * 200)
	}
	return nil, fmt.Errorf("timed out after %v waiting for %v", time.Since(start), name)
}

func TestMain(m *testing.M) {
	v23test.TestMain(m)
}
