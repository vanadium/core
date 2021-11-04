// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package scripting_test

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/x/ref/cmd/principal/internal/scripting"
	seclib "v.io/x/ref/lib/security"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/test"
	"v.io/x/ref/test/testutil"
)

func execute(t *testing.T, ctx *context.T, script string) string {
	out := strings.Builder{}
	scr := scripting.NewScript()
	scr.SetStdout(&out)
	err := scr.ExecuteBytes(ctx, []byte(script))
	if err != nil {
		_, _, line, _ := runtime.Caller(1)
		t.Fatalf("line %v: %v: %v", line, script, err)
	}
	return out.String()
}

func fail(t *testing.T, err error) {
	if err != nil {
		_, _, line, _ := runtime.Caller(1)
		t.Fatalf("line: %v: %v", line, err)
	}
}

func TestPrincipal(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()
	p := testutil.NewPrincipal("testing")

	ctx, _ = v23.WithPrincipal(ctx, p)

	out := execute(t, ctx, `p := defaultPrincipal()
	printf("principal:\n")
	printPrincipal(p)
	printf("publickey:\n")
	printPublicKey(p)
	b := getDefaultBlessings(p)
	printf("blessings:\n")
	printBlessings(b)
	`)
	fmt.Println(out)

	if got, want := strings.Count(out, p.PublicKey().String()), 5; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	if got, want := out, "Root certificate public key: "+p.PublicKey().String(); !strings.Contains(got, want) {
		t.Errorf("got %v does not contain %v", got, want)
	}

	b, err := p.BlessSelf("testing")
	fail(t, err)

	bstr, err := seclib.EncodeBlessingsBase64(b)
	fail(t, err)
	out = execute(t, ctx, `p := defaultPrincipal()
		`+fmt.Sprintf(`dec := decodeBlessingsBase64(%q)`, bstr)+
		`
		b := encodeBlessingsBase64(dec)
		printf("%v", b)
	`)
	if got, want := out, bstr; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	ssh := filepath.Join(t.TempDir(), "ssh")
	err = os.WriteFile(ssh, []byte(ecsdaOpenSSH), 0666)
	fail(t, err)
	//	sshpk, err := seclib.DecodePublicKeySSH([]byte(ecsdaOpenSSH))
	pk, err := seclib.DecodePublicKeySSH([]byte(ecsdaOpenSSH))
	fail(t, err)

	out = execute(t, ctx, `p := defaultPrincipal()
	`+fmt.Sprintf("other := decodePublicKeySSH(%q)", ssh)+`
	blessWith := createBlessings(p, "testing")
	expiresIn24h := expiryCaveat("24h")
	blessings := blessPrincipal(p, other, blessWith, "testing:other", expiresIn24h)
	printBlessings(blessings)
	`)
	fmt.Println("------------")
	fmt.Println(out)

	if got, want := out, "PublicKey          : "+pk.String(); !strings.Contains(got, want) {
		t.Errorf("got %v does not contain %v", got, want)
	}

	if got, want := out, `
  Certificate #0: testing with 0 caveats
  Certificate #1: testing:other with 1 caveat
`; !strings.Contains(got, want) {
		t.Errorf("got %v does not contain %v", got, want)
	}
}

const (
	ecsdaOpenSSH = `ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABBBJ/rseG6G7u0X1sIj3LK+hGtBEr4PQzrhCsHc0s4Wuso5j3Jxhcg5ze6MuxJqCRLtIOgIYTmY4K31wb3lHdtyGY= comment`
	ecsdaOpenSSL = `-----BEGIN PUBLIC KEY-----
MIGbMBAGByqGSM49AgEGBSuBBAAjA4GGAAQBnL/xs5AX1CDIkpmZWt4FJjpQIqid
m9poMZgdRIr7cKqkxy52th+oa/S//qXuhec5Dd8gvIBllbsWTXOCpWi4200Bk7nx
8ZcnmpkfT0pqoArEdEnWKEziQlIvdUZXIJ8qzrkdzDg8uhW6c1XBAcJQY9ohwOcA
wUsbRk1ox+ykM2ElKAU=
-----END PUBLIC KEY-----
`
)

func TestPublicKey(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()
	p := testutil.NewPrincipal("testing")
	ctx, _ = v23.WithPrincipal(ctx, p)

	b1, err := seclib.EncodePublicKeyBase64(p.PublicKey())
	fail(t, err)

	ssh := filepath.Join(t.TempDir(), "ssh")
	err = os.WriteFile(ssh, []byte(ecsdaOpenSSH), 0666)
	fail(t, err)
	ssl := filepath.Join(t.TempDir(), "ssl")
	err = os.WriteFile(ssl, []byte(ecsdaOpenSSL), 0666)
	fail(t, err)

	out := execute(t, ctx, `p := defaultPrincipal()
	`+fmt.Sprintf("k1 := decodePublicKeyBase64(%q)", b1)+`
	printf("%v\n",k1)
	`+fmt.Sprintf("k2 := decodePublicKeySSH(%q)", ssh)+`
	printf("%v\n",k2)
	`+fmt.Sprintf("k3 := decodePublicKeyPEM(%q)", ssl)+`
	printf("%v\n",k3)
	`+fmt.Sprintf("md5sig := sshPublicKeyMD5(%q)", ssh)+`
		printf("%v\n", md5sig)
	`+fmt.Sprintf("sha256sig := sshPublicKeySHA256(%q)", ssh)+`
	printf("%v\n", sha256sig)
`)

	if got, want := out, p.PublicKey().String()+`
3f:8a:b6:38:a6:33:d0:eb:62:e5:50:31:2e:75:81:78
d4:80:01:46:ec:34:01:23:34:19:2b:e6:6e:29:14:e8
76:84:f1:22:8c:76:c0:1e:d3:d3:2c:9d:0c:a3:1d:1f
QkzGB1AoO4Oy5yKtZ2VDLP6v5sWCzcBsg4rlzhiCoEQ
`; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

}

func TestSSLKeys(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()
	p := testutil.NewPrincipal("testing")
	kfile := filepath.Join("testdata", "rsa2048.vanadium.io.key")
	pdir := filepath.Join(t.TempDir(), "ssl-principal")
	ctx, _ = v23.WithPrincipal(ctx, p)
	out := execute(t, ctx,
		fmt.Sprintf("key := useSSLKey(%q)\n", kfile)+
			fmt.Sprintf("p := useOrCreatePrincipal(key, %q)\n", pdir)+
			"printPrincipal(p)")
	if got, want := out, `Public key : 53:fb:b2:07:10:fd:9c:89:16:f5:76:4b:e8:5c:17:30
Default Blessings : 
---------------- BlessingStore ----------------
Default Blessings                
Peer pattern                     Blessings
---------------- BlessingRoots ----------------
Public key                                        Pattern
`; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestCaveats(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	out := execute(t, ctx, `
	deny := denyAllCaveat()
	allow := allowAllCaveat()
	m1 := methodCaveat("m1")
	m12 := methodCaveat("m1", "m2")
	nyt := parseTime(time_RFC822Z, "12 Jan 20 17:00 -0500")
	when := deadline(nyt, "1h")
	d2 := deadlineCaveat(when)
	cavs := formatCaveats(deny, allow, m1, m12, d2)
	printf("%v\n", cavs)
	`)
	if got, want := out, `Never validates
Always validates
Restricted to methods [m1]
Restricted to methods [m2 m1]
Expires at 2020-01-12 23:00:00 +0000 UTC
`; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	ssl := filepath.Join(t.TempDir(), "ssl")
	err := os.WriteFile(ssl, []byte(ecsdaOpenSSL), 0666)
	fail(t, err)

	out = execute(t, ctx, `
	c3reqs := thirdPartyCaveatRequirements(true, true, false)
	`+fmt.Sprintf("dischargerPK := decodePublicKeyPEM(%q)", ssl)+`
	nyt := parseTime(time_RFC822Z, "12 Jan 20 17:00 -0500")
	until := expiryCaveat("1h")
	c3cav := publicKeyCaveat(dischargerPK, "somewhere", c3reqs, until)
	cavs := formatCaveats(c3cav)
	printf("%v\n", cavs)
	`)

	if got, want := out, "ThirdPartyCaveat: Requires discharge from somewhere"; !strings.Contains(got, want) {
		t.Errorf("got %v does not contain %v", got, want)
	}
}
