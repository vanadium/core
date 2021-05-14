// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"os/user"
	"strings"
	"time"

	"v.io/v23/security"
	"v.io/v23/vom"
)

func DumpBlessingsFile(out io.Writer, filename string) error {
	blessings, err := DecodeBlessingsFile(filename)
	if err != nil {
		return fmt.Errorf("failed to decode provided blessings: %v", err)
	}
	return DumpBlessings(out, blessings)
}

func DumpBlessings(out io.Writer, blessings security.Blessings) error {
	wire, err := blessings2wire(blessings)
	if err != nil {
		return fmt.Errorf("failed to decode certificate chains: %v", err)
	}
	fmt.Fprintf(out, "Blessings          : %s\n", annotatedBlessingsNames(blessings))
	fmt.Fprintf(out, "PublicKey          : %v\n", blessings.PublicKey())
	fmt.Fprintf(out, "Certificate chains : %d\n", len(wire.CertificateChains))
	for idx, chain := range wire.CertificateChains {
		fmt.Fprintf(out, "Chain #%d (%d certificates). Root certificate public key: %v\n", idx, len(chain), Rootkey(chain))
		dumpCertChain(out, "  ", chain, false)
	}
	return nil
}

func dumpCertChain(out io.Writer, indent string, chain []security.Certificate, pk bool) error {
	for certidx, cert := range chain {
		fmt.Fprintf(out, "%sCertificate #%d: %v with ", indent, certidx, cert.Extension)
		if pk {
			key, err := security.UnmarshalPublicKey(cert.PublicKey)
			if err != nil {
				return fmt.Errorf("cert #%d: <invalid PublicKey: %v>", certidx, err)
			}
			fmt.Fprintf(out, "%sCertificate #%d: %v: ", indent, certidx, key)
		}
		switch n := len(cert.Caveats); n {
		case 1:
			fmt.Fprintf(out, "1 caveat")
		default:
			fmt.Fprintf(out, "%d caveats", n)
		}
		fmt.Fprintln(out, "")
		for cavidx, cav := range cert.Caveats {
			fmt.Fprintf(out, "%s  (%d) %v\n", indent, cavidx, cav.String())
		}
	}
	return nil
}

func DumpCertificateChain(out io.Writer, chain []security.Certificate) error {
	return dumpCertChain(out, "", chain, true)
}

func DumpPublicKey(out io.Writer, p security.Principal, pretty bool) error {
	key := p.PublicKey()
	if pretty {
		fmt.Fprintln(out, key)
		return nil
	}
	der, err := key.MarshalBinary()
	if err != nil {
		return fmt.Errorf("corrupted key: %v", err)
	}
	fmt.Fprintln(out, base64.URLEncoding.EncodeToString(der))
	return nil
}

func DumpPrincipal(out io.Writer, p security.Principal, blessingNamesOnly bool) error {
	def, _ := p.BlessingStore().Default()
	if blessingNamesOnly {
		fmt.Fprintf(out, "%s\n", annotatedBlessingsNames(def))
		return nil
	}
	fmt.Fprintf(out, "Public key : %v\n", p.PublicKey())
	// NOTE(caprita): We print the default blessings name
	// twice (it's also printed as part of the blessing
	// store below) -- the reason we print it here is to
	// expose whether the blessings are expired.  Ideally,
	// the blessings store would print the expiry
	// information about each blessing in the store, but
	// that would require deeper changes beyond the
	// principal tool.
	fmt.Fprintf(out, "Default Blessings : %s\n", annotatedBlessingsNames(def))
	fmt.Fprintln(out, "---------------- BlessingStore ----------------")
	fmt.Fprintf(out, "%v", p.BlessingStore().DebugString())
	fmt.Fprintln(out, "---------------- BlessingRoots ----------------")
	fmt.Fprintf(out, "%v", p.Roots().DebugString())
	return nil
}

func annotatedBlessingsNames(b security.Blessings) string {
	// If the Blessings are expired, print a message saying so.
	expiredMessage := ""
	if exp := b.Expiry(); !exp.IsZero() && exp.Before(time.Now()) {
		expiredMessage = " [EXPIRED]"
	}
	return fmt.Sprintf("%v%s", b, expiredMessage)
}

func Rootkey(chain []security.Certificate) string {
	if len(chain) == 0 {
		return "<empty certificate chain>"
	}
	key, err := security.UnmarshalPublicKey(chain[0].PublicKey)
	if err != nil {
		return fmt.Sprintf("<invalid PublicKey: %v>", err)
	}
	return fmt.Sprintf("%v", key)
}

func FormatCaveatsInChain(chain []security.Certificate) ([]string, error) {
	var cavs []security.Caveat
	for _, cert := range chain {
		cavs = append(cavs, cert.Caveats...)
	}
	return FormatCaveats(cavs)
}

func FormatCaveats(cavs []security.Caveat) ([]string, error) {
	var s []string
	for _, cav := range cavs {
		if cav.Id == security.PublicKeyThirdPartyCaveat.Id {
			c := cav.ThirdPartyDetails()
			s = append(s, fmt.Sprintf("ThirdPartyCaveat: Requires discharge from %v (ID=%q)", c.Location(), c.ID()))
			continue
		}
		var param interface{}
		if err := vom.Decode(cav.ParamVom, &param); err != nil {
			return nil, err
		}
		switch cav.Id {
		case security.ConstCaveat.Id:
			// In the case a ConstCaveat is specified, we only want to print it
			// if it never validates.
			if !param.(bool) {
				s = append(s, "Never validates")
			}
		case security.ExpiryCaveat.Id:
			s = append(s, fmt.Sprintf("Expires at %v", param))
		case security.MethodCaveat.Id:
			s = append(s, fmt.Sprintf("Restricted to methods %v", param))
		case security.PeerBlessingsCaveat.Id:
			s = append(s, fmt.Sprintf("Restricted to peers with blessings %v", param))
		default:
			s = append(s, cav.String())
		}
	}
	return s, nil
}

func ChainName(chain []security.Certificate) string {
	exts := make([]string, len(chain))
	for i, cert := range chain {
		exts[i] = cert.Extension
	}
	return strings.Join(exts, security.ChainSeparator)
}

func GetChainByName(b security.Blessings, name string) ([]security.Certificate, error) {
	wire, err := blessings2wire(b)
	if err != nil {
		return nil, err
	}
	for _, chain := range wire.CertificateChains {
		if ChainName(chain) == name {
			return chain, nil
		}
	}
	return nil, fmt.Errorf("no chains of name %v in %v", name, b)
}

// CreateDefaultBlessingName generates a blessing name based on the hostname
// of the machine and the name of the user running this command.
func CreateDefaultBlessingName() string {
	var name string
	if user, _ := user.Current(); user != nil && len(user.Username) > 0 {
		name = user.Username
	} else {
		name = "anonymous"
	}
	if host, _ := os.Hostname(); len(host) > 0 {
		name = name + "@" + host
	}
	return name
}
