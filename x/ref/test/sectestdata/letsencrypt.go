// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sectestdata

import (
	"crypto"
	"crypto/x509"
	"embed"
	_ "embed"
	"fmt"
)

const (
	letsEncryptStagingE1 = "letsencrypt-stg-int-e1.pem"
	letsEncryptStagingR3 = "letsencrypt-stg-int-r3.pem"
)

//go:embed testdata/letsencrypt-stg-int-e1.pem testdata/www.labdrive.io.letsencrypt testdata/www.labdrive.io.letsencrypt.key
var letsEncryptSingleHostFS embed.FS

//go:embed testdata/letsencrypt-stg-int-r3.pem testdata/star.labdrive.io.letsencrypt testdata/star.labdrive.io.letsencrypt.key
var letsEncryptWildcardFS embed.FS

//go:embed testdata/letsencrypt-stg-int-r3.pem testdata/abc.labdrive.io.letsencrypt testdata/abc.labdrive.io.letsencrypt.key
var letsEncryptMultiHostFS embed.FS

// CertType specifies the type of cert to be used.
type CertType int

// Supported cert types are below.
const (
	// SingleHostCert refers to a cert and key for www.labdrive.io
	SingleHostCert CertType = iota
	// MultipleHostsCert refers to a cert and key for {a,b,c}.labdrive.io
	MultipleHostsCert
	// WildcardCert refers to a cert and key for *.labdrive.io
	WildcardCert
)

func (c CertType) String() string {
	switch c {
	case SingleHostCert:
		return "single-host-cert"
	case MultipleHostsCert:
		return "multi-host-san-cert"
	case WildcardCert:
		return "wildcard-cert"
	}
	panic("wrong cert type")
}

// LetsencryptData returns an SSL private key and certificate obtained from
// letsencrypt's staging environment. The returned VerifyOptions contain a cert
// pool and current time that will successfully validate that certificate.
func LetsEncryptData(certType CertType) (crypto.PrivateKey, []*x509.Certificate, x509.VerifyOptions) {
	switch certType {
	case SingleHostCert:
		return letsEncryptData(letsEncryptSingleHostFS, "www.labdrive.io.letsencrypt.key", "www.labdrive.io.letsencrypt", letsEncryptStagingE1)
	case MultipleHostsCert:
		return letsEncryptData(letsEncryptMultiHostFS, "abc.labdrive.io.letsencrypt.key", "abc.labdrive.io.letsencrypt", letsEncryptStagingR3)
	case WildcardCert:
		return letsEncryptData(letsEncryptWildcardFS, "star.labdrive.io.letsencrypt.key", "star.labdrive.io.letsencrypt", letsEncryptStagingR3)
	default:
		panic(fmt.Sprintf("unsupported cert type: %v", certType))
	}
}

func letsEncryptData(fs embed.FS, key, cert, ca string) (crypto.PrivateKey, []*x509.Certificate, x509.VerifyOptions) {
	return loadCertSet(
		mustBytesFromFS(fs, "testdata", key),
		mustBytesFromFS(fs, "testdata", cert),
		mustBytesFromFS(fs, "testdata", ca),
	)
}

// LetsencryptDir creates a pre-populated directory with letsencrypt
// derived files for the requested cert type.
func LetsEncryptDir(certType CertType) (string, error) {
	switch certType {
	case SingleHostCert:
		return prepopulatedDir(letsEncryptSingleHostFS, "www-labdrive", "testdata")
	case MultipleHostsCert:
		return prepopulatedDir(letsEncryptMultiHostFS, "abc-labdrive", "testdata")
	case WildcardCert:
		return prepopulatedDir(letsEncryptWildcardFS, "star-labdrive", "testdata")
	default:
		panic(fmt.Sprintf("unsupported cert type: %v", certType))
	}
}
