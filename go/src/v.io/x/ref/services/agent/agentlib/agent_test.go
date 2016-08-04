// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package agentlib_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"v.io/v23"
	"v.io/v23/security"
	"v.io/x/lib/gosh"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/services/agent/agentlib"
	"v.io/x/ref/services/agent/internal/ipc"
	"v.io/x/ref/services/agent/internal/server"
	"v.io/x/ref/test"
	"v.io/x/ref/test/testutil"
	"v.io/x/ref/test/v23test"
)

// As of November 17, 2015, the benchmarks for serving a principal with and
// without the agent are as follows:
// (on an 3.1 GHz Intel Core i7)
//
// BenchmarkSignNoAgent-4                    	    2000	    625858 ns/op
// BenchmarkSignCachedAgent-4                	    2000	    805983 ns/op
// BenchmarkSignUncachedAgent-4              	    2000	    789841 ns/op
// BenchmarkDefaultNoAgent-4                 	20000000	       109 ns/op
// BenchmarkDefaultCachedAgent-4             	30000000	        44.9 ns/op
// BenchmarkDefaultUncachedAgent-4           	   10000	    155312 ns/op
// BenchmarkRecognizedNegativeNoAgent-4      	  100000	     20091 ns/op
// BenchmarkRecognizedNegativeCachedAgent-4  	  100000	     21940 ns/op
// BenchmarkRecognizedNegativeUncachedAgent-4	   10000	    209605 ns/op
// BenchmarkRecognizedNoAgent-4              	 2000000	       748 ns/op
// BenchmarkRecognizedCachedAgent-4          	 3000000	       774 ns/op
// BenchmarkRecognizedUncachedAgent-4        	   20000	     99022 ns/op

var getPrincipalAndHang = gosh.RegisterFunc("getPrincipalAndHang", func() {
	ctx, shutdown := test.V23Init()
	defer shutdown()
	b, _ := v23.GetPrincipal(ctx).BlessingStore().Default()
	fmt.Printf("DEFAULT_BLESSING=%s\n", b)
	ioutil.ReadAll(os.Stdin)
})

func newAgent(path string, cached bool) (security.Principal, error) {
	if cached {
		return agentlib.NewAgentPrincipalX(path)
	} else {
		return agentlib.NewUncachedPrincipalX(path)
	}
}

func setupAgentPair(t *testing.T, p security.Principal) (security.Principal, security.Principal, func()) {
	i := ipc.NewIPC()
	if err := server.ServeAgent(i, p); err != nil {
		t.Fatal(err)
	}
	dir, err := ioutil.TempDir("", "agent")
	if err != nil {
		t.Fatal(err)
	}
	sock := filepath.Join(dir, "sock")
	defer os.RemoveAll(dir)
	if err := i.Listen(sock); err != nil {
		t.Fatal(err)
	}
	agent1, err := newAgent(sock, true)
	if err != nil {
		t.Fatal(err)
	}
	agent2, err := newAgent(sock, true)
	if err != nil {
		t.Fatal(err)
	}

	return agent1, agent2, i.Close
}

func setupAgent(caching bool) (security.Principal, func()) {
	p := testutil.NewPrincipal("agentTest")
	i := ipc.NewIPC()
	if err := server.ServeAgent(i, p); err != nil {
		panic(err)
	}
	dir, err := ioutil.TempDir("", "agent")
	if err != nil {
		panic(err)
	}
	sock := filepath.Join(dir, "sock")
	defer os.RemoveAll(dir)
	if err := i.Listen(sock); err != nil {
		panic(err)
	}

	agent, err := newAgent(sock, caching)
	if err != nil {
		panic(err)
	}
	return agent, i.Close
}

func TestAgent(t *testing.T) {
	var (
		p                       = testutil.NewPrincipal("agentTest")
		agent1, agent2, cleanup = setupAgentPair(t, p)
		def                     = func(p security.Principal) security.Blessings {
			b, _ := p.BlessingStore().Default()
			return b
		}
		defP = def(p)
		def1 = def(agent1)
		def2 = def(agent2)
	)
	defer cleanup()

	if !reflect.DeepEqual(defP, def1) {
		t.Errorf("Default blessing mismatch. Wanted %v, got %v", defP, def1)
	}
	if !reflect.DeepEqual(defP, def2) {
		t.Errorf("Default blessing mismatch. Wanted %v, got %v", defP, def2)
	}

	// Check that we're caching:
	// Modify the principal directly and the client's shouldn't notic.
	blessing, err := p.BlessSelf("alice")
	if err != nil {
		t.Fatal(err)
	}
	if err = p.BlessingStore().SetDefault(blessing); err != nil {
		t.Fatal(err)
	}
	def1, def2 = def(agent1), def(agent2)
	if !reflect.DeepEqual(defP, def1) {
		t.Errorf("Default blessing mismatch. Wanted %v, got %v", defP, def1)
	}
	if !reflect.DeepEqual(defP, def2) {
		t.Errorf("Default blessing mismatch. Wanted %v, got %v", defP, def2)
	}

	// Now make a change through the client.
	blessing, err = agent1.BlessSelf("john")
	if err != nil {
		t.Fatal(err)
	}
	if err = agent2.BlessingStore().SetDefault(blessing); err != nil {
		t.Fatal(err)
	}
	// The principal and the other client should both reflect the change.
	if newDefault := def(p); !reflect.DeepEqual(newDefault, blessing) {
		t.Errorf("Default blessing mismatch. Wanted %v, got %v", blessing, newDefault)
	}
	// There's no synchronization, so keep fetching from the other client.
	// Eventually it should get notified of the new value.
	for i := 0; i < 10000 && !reflect.DeepEqual(blessing, def(agent1)); i += 1 {
		time.Sleep(100 * time.Millisecond)
	}

	if got := def(agent1); !reflect.DeepEqual(got, blessing) {
		t.Errorf("Default blessing mismatch. Wanted %v, got %v", blessing, got)
	}
}

func TestAgentDischargeCache(t *testing.T) {
	agent, cleanup := setupAgent(true)
	defer cleanup()

	expcav, err := security.NewExpiryCaveat(time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	tpcav, err := security.NewPublicKeyCaveat(
		agent.PublicKey(),
		"discharger",
		security.ThirdPartyRequirements{},
		expcav)
	d, err := agent.MintDischarge(tpcav, expcav)
	if err != nil {
		t.Fatal(err)
	}
	before := time.Now()
	agent.BlessingStore().CacheDischarge(d, tpcav, security.DischargeImpetus{})
	after := time.Now()
	got, cacheTime := agent.BlessingStore().Discharge(tpcav, security.DischargeImpetus{})
	if d.ID() != got.ID() {
		t.Errorf("got: %v, wanted %v", got.ID(), d.ID())
	}
	if cacheTime.Before(before) || cacheTime.After(after) {
		t.Errorf("got %v, wanted value in (%v, %v)", cacheTime, before, after)
	}
}

// NOTE(sadovsky): Whether v23test.Shell uses the agent to manage credentials is
// an internal implementation detail of the Shell. Currently, it happens to use
// the agent. We could potentially expose an option to allow clients to
// configure this behavior.
func TestAgentShutdown(t *testing.T) {
	sh := v23test.NewShell(t, nil)
	defer func() {
		fmt.Fprintf(os.Stderr, "cleanup...\n")
		sh.Cleanup()
		fmt.Fprintf(os.Stderr, "cleanup done\n")
	}()

	// The child process will connect to the agent.
	c := sh.FuncCmd(getPrincipalAndHang)
	c.Start()

	fmt.Fprintf(os.Stderr, "reading var...\n")
	c.S.ExpectVar("DEFAULT_BLESSING")
	fmt.Fprintf(os.Stderr, "read\n")
	if err := c.S.Error(); err != nil {
		t.Fatalf("failed to read input: %s", err)
	}
}

var message = []byte("bugs bunny")

func runSignBenchmark(b *testing.B, p security.Principal) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := p.Sign(message); err != nil {
			b.Fatal(err)
		}
	}
}

func runDefaultBenchmark(b *testing.B, p security.Principal) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if d, _ := p.BlessingStore().Default(); d.IsZero() {
			b.Fatal("empty blessings")
		}
	}
}

func runRecognizedNegativeBenchmark(b *testing.B, p security.Principal) {
	key, err := p.PublicKey().MarshalBinary()
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if d := p.Roots().Recognized(key, "foobar"); d == nil {
			b.Fatal("nil")
		}
	}
}

func runRecognizedBenchmark(b *testing.B, p security.Principal) {
	key, err := p.PublicKey().MarshalBinary()
	if err != nil {
		b.Fatal(err)
	}
	blessing, err := p.BlessSelf("foobar")
	if err != nil {
		b.Fatal(err)
	}
	if err = security.AddToRoots(p, blessing); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if d := p.Roots().Recognized(key, "foobar"); d != nil {
			b.Fatal(d)
		}
	}
}

func BenchmarkSignNoAgent(b *testing.B) {
	p := testutil.NewPrincipal("agentTest")
	runSignBenchmark(b, p)
}

func BenchmarkSignCachedAgent(b *testing.B) {
	p, cleanup := setupAgent(true)
	defer cleanup()
	runSignBenchmark(b, p)
}

func BenchmarkSignUncachedAgent(b *testing.B) {
	p, cleanup := setupAgent(false)
	defer cleanup()
	runSignBenchmark(b, p)
}

func BenchmarkDefaultNoAgent(b *testing.B) {
	p := testutil.NewPrincipal("agentTest")
	runDefaultBenchmark(b, p)
}

func BenchmarkDefaultCachedAgent(b *testing.B) {
	p, cleanup := setupAgent(true)
	defer cleanup()
	runDefaultBenchmark(b, p)
}

func BenchmarkDefaultUncachedAgent(b *testing.B) {
	p, cleanup := setupAgent(false)
	defer cleanup()
	runDefaultBenchmark(b, p)
}

func BenchmarkRecognizedNegativeNoAgent(b *testing.B) {
	p := testutil.NewPrincipal("agentTest")
	runRecognizedNegativeBenchmark(b, p)
}

func BenchmarkRecognizedNegativeCachedAgent(b *testing.B) {
	p, cleanup := setupAgent(true)
	defer cleanup()
	runRecognizedNegativeBenchmark(b, p)
}

func BenchmarkRecognizedNegativeUncachedAgent(b *testing.B) {
	p, cleanup := setupAgent(false)
	defer cleanup()
	runRecognizedNegativeBenchmark(b, p)
}

func BenchmarkRecognizedNoAgent(b *testing.B) {
	p := testutil.NewPrincipal("agentTest")
	runRecognizedBenchmark(b, p)
}

func BenchmarkRecognizedCachedAgent(b *testing.B) {
	p, cleanup := setupAgent(true)
	defer cleanup()
	runRecognizedBenchmark(b, p)
}

func BenchmarkRecognizedUncachedAgent(b *testing.B) {
	p, cleanup := setupAgent(false)
	defer cleanup()
	runRecognizedBenchmark(b, p)
}
