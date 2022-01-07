// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sectestdata

import (
	"crypto"
	"crypto/x509"
	"embed"
	_ "embed"
)

//go:embed testdata/letsencrypt-stg-int-e1.pem
var letsEncryptStagingCA []byte

//go:embed testdata/www.labdrive.io.letsencrypt
var labDriveCert []byte

//go:embed testdata/www.labdrive.io.letsencrypt.key
var labDriveKey []byte

// LetsencryptData returns an SSL private key and certificate obtained from
// letsencrypt's staging environment. The returned VerifyOptions contain a cert
// pool and current time that will successfully validate that certificate.
func LetsEncryptData() (crypto.PrivateKey, []*x509.Certificate, x509.VerifyOptions) {
	key, err := loadPrivateKey(labDriveKey)
	if err != nil {
		panic(err)
	}
	certs, err := loadCerts(labDriveCert)
	if err != nil {
		panic(err)
	}
	opts, err := loadCA(certs[0], letsEncryptStagingCA)
	if err != nil {
		panic(err)
	}
	return key, certs, opts
}

//go:embed testdata/letsencrypt-stg-int-e1.pem testdata/www.labdrive.io.letsencrypt testdata/www.labdrive.io.letsencrypt.key
var letsEncryptFS embed.FS

// LetsencryptDir creates a pre-populated directory with letsencrypt
// derived files:
func LetsEncryptDir() (string, error) {
	return prepopulatedDir("letsencrypt", "testdata", letsEncryptFS)
}
