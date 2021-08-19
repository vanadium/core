package security

import (
	"net/url"
	"testing"

	"v.io/v23/security"
)

func TestBlessingsBase64(t *testing.T) {
	p, err := NewPrincipal()
	if err != nil {
		t.Fatal(err)
	}
	tester := newStoreTester(p)

	for _, blessing := range []security.Blessings{
		tester.forAll, tester.forFoo, tester.forBar, tester.def, tester.other,
	} {
		enc, err := EncodeBlessingsBase64(blessing)
		if err != nil {
			t.Errorf("encode: %v: %v", blessing, err)
			continue
		}
		dec, err := DecodeBlessingsBase64(enc)
		if err != nil {
			t.Errorf("decode: %v: %v", blessing, err)
			continue
		}
		if got, want := dec.String(), blessing.String(); got != want {
			t.Errorf("got %v, want %v", got, want)
		}
		if got, want := dec, blessing; !got.Equivalent(want) {
			t.Errorf("got %v not equivalent to want %v", got, want)
		}
		pq, err := url.ParseQuery("blessings=" + enc)
		if err != nil {
			t.Errorf("parseQuery: %v: %v", blessing, err)
			continue
		}
		dec, err = DecodeBlessingsBase64(pq.Get("blessings"))
		if err != nil {
			t.Errorf("decode query: %v: %v", blessing, err)
			continue
		}
		if got, want := dec.String(), blessing.String(); got != want {
			t.Errorf("got %v, want %v", got, want)
		}
		if got, want := dec, blessing; !got.Equivalent(want) {
			t.Errorf("got %v not equivalent to want %v", got, want)
		}
	}

	all, err := security.UnionOfBlessings(tester.forAll, tester.forFoo, tester.forBar, tester.def)
	if err != nil {
		t.Fatal(err)
	}
	enc, err := EncodeBlessingsBase64(all)
	if err != nil {
		t.Fatalf("encode: %v: %v", all, err)
	}
	dec, err := DecodeBlessingsBase64(enc)
	if err != nil {
		t.Fatalf("decode: %v: %v", all, err)

	}
	if got, want := dec, all; !got.Equivalent(want) {
		t.Errorf("got %v not equivalent to want %v", got, want)
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

func TestPublicKeys(t *testing.T) {
	p, err := NewPrincipal()
	if err != nil {
		t.Fatal(err)
	}
	buf, err := EncodePublicKeyBase64(p.PublicKey())
	if err != nil {
		t.Fatal(err)
	}

	k, err := DecodePublicKeyBase64(buf)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := p.PublicKey().String(), k.String(); got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	pk, err := DecodePublicKeySSH([]byte(ecsdaOpenSSH))
	if err != nil {
		t.Fatal(err)
	}

	sigmd5, err := SSHSignatureMD5([]byte(ecsdaOpenSSH))
	if err != nil {
		t.Fatal(err)
	}

	sigsha256, err := SSHSignatureSHA256([]byte(ecsdaOpenSSH))
	if err != nil {
		t.Fatal(err)
	}

	// NOTE: generating the fingerprint is a little annoying with ssh and openssl
	//       since we need to generate it over the PKIX DER format which
	//       requires the following steps:
	// ssh-keygen -t ecdsa -f key -m pem
	// openssl ec -in key --inform PEM --outform DER --pubout | openssl md5 -c
	if got, want := pk.String(), "3f:8a:b6:38:a6:33:d0:eb:62:e5:50:31:2e:75:81:78"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := sigmd5, "76:84:f1:22:8c:76:c0:1e:d3:d3:2c:9d:0c:a3:1d:1f"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := sigsha256, "QkzGB1AoO4Oy5yKtZ2VDLP6v5sWCzcBsg4rlzhiCoEQ"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	// openssl ecparam -name secp521r1 -genkey -noout | openssl ec --pubout > key
	// cat key | openssl ec --inform PEM --outform DER --pubin | openssl md5 -c
	pk, err = DecodePublicKeyPEM([]byte(ecsdaOpenSSL))
	if err != nil {
		t.Fatal(err)
	}

	if got, want := pk.String(), "d4:80:01:46:ec:34:01:23:34:19:2b:e6:6e:29:14:e8"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

}
