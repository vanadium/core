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
	"v.io/x/ref/lib/security/keys"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/test"
	"v.io/x/ref/test/sectestdata"
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

	sshFile := filepath.Join(t.TempDir(), "ssh")
	pkBytes := sectestdata.SSHPublicKeyBytes(keys.ECDSA256, sectestdata.SSHKeyPublic)
	err = os.WriteFile(sshFile, pkBytes, 0666)
	fail(t, err)

	pk, err := seclib.ParsePublicKey(pkBytes)
	fail(t, err)
	api, err := seclib.APIForKey(pk)
	fail(t, err)
	sshPk, err := api.PublicKey(pk)
	fail(t, err)

	out = execute(t, ctx, `p := defaultPrincipal()
	`+fmt.Sprintf("other := readPublicKey(%q)", sshFile)+`
	blessWith := createBlessings(p, "testing")
	expiresIn24h := expiryCaveat("24h")
	blessings := blessPrincipal(p, other, blessWith, "testing:other", expiresIn24h)
	printBlessings(blessings)
	`)
	fmt.Println("------------")
	fmt.Println(out)

	if got, want := out, "PublicKey          : "+sshPk.String(); !strings.Contains(got, want) {
		t.Errorf("got %v does not contain %v", got, want)
	}

	if got, want := out, `
  Certificate #0: testing with 0 caveats
  Certificate #1: testing:other with 1 caveat
`; !strings.Contains(got, want) {
		t.Errorf("got %v does not contain %v", got, want)
	}
}

func TestPublicKey(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()
	p := testutil.NewPrincipal("testing")
	ctx, _ = v23.WithPrincipal(ctx, p)

	b1, err := seclib.EncodePublicKeyBase64(p.PublicKey())
	fail(t, err)

	ssh := filepath.Join(t.TempDir(), "ssh")
	err = os.WriteFile(ssh, sectestdata.SSHPublicKeyBytes(keys.ECDSA256, sectestdata.SSHKeyPublic), 0666)
	fail(t, err)

	out := execute(t, ctx, fmt.Sprintf("k1 := decodePublicKeyBase64(%q)", b1)+`
	printf("%v\n",k1)
	`+fmt.Sprintf("md5sig := sshPublicKeyMD5(%q)", ssh)+`
		printf("%v\n", md5sig)
	`+fmt.Sprintf("sha256sig := sshPublicKeySHA256(%q)", ssh)+`
	printf("%v\n", sha256sig)
`)

	if got, want := out, p.PublicKey().String()+`
ec:12:a7:ce:7f:b6:bb:00:10:0c:da:93:bc:7b:e9:a1
SHA256:ae9Iul6Y+1aInM44SBFYco1I/sXsidGD5apGk2gc54I
`; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

}

func TestExistingKeys(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	p := testutil.NewPrincipal("testing")

	for _, tc := range []struct {
		data        []byte
		fingerprint string
	}{
		{
			sectestdata.X509PrivateKeyBytes(keys.ED25519, sectestdata.X509Private),
			"7c:06:35:de:48:75:41:8c:71:f3:e9:e0:44:98:4a:d3",
		},
		{
			sectestdata.SSHPrivateKeyBytes(keys.ECDSA256, sectestdata.SSHKeyPrivate),
			"12:e0:1e:70:14:5a:b3:01:c4:8e:44:96:df:9c:2d:ba",
		},
	} {
		tdir := t.TempDir()
		kfile := filepath.Join(tdir, "keyfile")
		if err := os.WriteFile(kfile, tc.data, 0600); err != nil {
			t.Fatal(err)
		}
		pdir := filepath.Join(tdir, "principal")
		ctx, _ = v23.WithPrincipal(ctx, p)
		out := execute(t, ctx,
			fmt.Sprintf("key := useExistingPrivateKey(%q)\n", kfile)+
				fmt.Sprintf("p := useOrCreatePrincipal(key, %q)\n", pdir)+
				"printPrincipal(p)")
		if got, want := out, fmt.Sprintf(`Public key : %s
Default Blessings : 
---------------- BlessingStore ----------------
Default Blessings                
Peer pattern                     Blessings
---------------- BlessingRoots ----------------
Public key                                        Pattern
`, tc.fingerprint); got != want {
			t.Errorf("got %v, want %v", got, want)
		}
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

	pubKeyFile := filepath.Join(t.TempDir(), "ssl")
	err := os.WriteFile(pubKeyFile, sectestdata.X509PublicKeyBytes(keys.ED25519), 0666)
	fail(t, err)

	out = execute(t, ctx, `
	c3reqs := thirdPartyCaveatRequirements(true, true, false)
	`+fmt.Sprintf("dischargerPK := readPublicKey(%q)", pubKeyFile)+`
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
