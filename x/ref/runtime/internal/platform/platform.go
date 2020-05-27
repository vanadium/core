// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package platform

import (
	"fmt"

	"v.io/v23/security"
)

// Platform describes the hardware and software environment that this
// process is running on. It is modeled on the Unix uname call.
type Platform struct {
	// Vendor is the manufacturer of the system. It must be possible
	// to crypographically verify the authenticity of the Vendor
	// using the value of Vendor. The test must fail if the underlying
	// hardware does not provide appropriate support for this.
	// TODO(ashankar, ataly): provide an API for verifying vendor authenticity.
	Vendor string

	// AIKCertificate attests that the platform contains a TPM that is trusted
	// and has valid EK (Endorsement Key) and platform credentials. The
	// AIKCertificate is bound to an AIK public key (attestation identity key)
	// that is secure on the TPM and can be used for signing operations.
	// TODO(gauthamt): provide an implementation of how a device gets this
	// certificate and how a remote process uses it to verify device identity.
	AIKCertificate *security.Certificate

	// Model is the model description, including version information.
	Model string

	// System is the name of the operating system.
	// E.g. 'Linux', 'Darwin'
	System string

	// Release is the specific release of System if known, the empty
	// string otherwise.
	// E.g. 3.12.24-1-ARCH on a Raspberry pi running Arch Linux
	Release string

	// Version is the version of System if known, the empty string otherwise.
	// E.g. #1 PREEMPT Thu Jul 10 23:57:15 MDT 2014 on a Raspberry Pi B
	// running Arch Linux
	Version string

	// Machine is the hardware identifier
	// E.g. armv6l on a Raspberry Pi B
	Machine string

	// Node is the name of the name of the node that the
	// the platform is running on. This is not necessarily unique.
	Node string
}

func (p *Platform) String() string {
	return fmt.Sprintf("%s/%s node %s running %s (%s %s) on machine %s", p.Vendor, p.Model, p.Node, p.System, p.Release, p.Version, p.Machine)
}
