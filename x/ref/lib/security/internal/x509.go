// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"bufio"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"regexp"
	"strings"
)

// ReadPEMBlocks returns all PEM blocks that match the specified type.
func ReadPEMBlocks(rd io.Reader, pemType *regexp.Regexp) ([]*pem.Block, error) {
	sc := bufio.NewScanner(rd)
	block := make([]byte, 0, 2048)

	appendBlock := func() {
		cpy := make([]byte, len(sc.Bytes()))
		copy(cpy, sc.Bytes())
		block = append(block, cpy...)
		block = append(block, '\n')
	}
	inblock := false
	pemBlocks := make([]*pem.Block, 0, 150)
	for sc.Scan() {
		l := sc.Text()
		switch {
		case strings.HasPrefix(l, "-----BEGIN ") && strings.HasSuffix(l, "-----"):
			inblock = true
			block = block[:0]
		case strings.HasPrefix(l, "-----END ") && strings.HasSuffix(l, "-----"):
			appendBlock()
			pemBlock, rest := pem.Decode(block)
			if pemBlock != nil && pemType.MatchString(pemBlock.Type) {
				pemBlocks = append(pemBlocks, pemBlock)
			}
			if len(rest) != 0 {
				return nil, fmt.Errorf("failed to certificate PEM block as a single pem block")
			}
			inblock = false
		}
		if inblock {
			appendBlock()
		}
	}
	return pemBlocks, nil
}

// ParseX509Certificates parses the supplied PEM blocks for x509 Certificates.
func ParseX509Certificates(pemBlocks []*pem.Block) ([]*x509.Certificate, error) {
	certs := make([]*x509.Certificate, 0, len(pemBlocks))
	for _, block := range pemBlocks {
		if block.Type != certPEMType {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return certs, err
		}
		certs = append(certs, cert)
	}
	return certs, nil
}
