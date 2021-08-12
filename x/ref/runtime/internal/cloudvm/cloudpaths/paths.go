// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cloudpaths

// AWS constants for its EC2 metadata service.
const (
	AWSHost            = "http://169.254.169.254"
	AWSTokenPath       = "/latest/api/token"
	AWSIdentityDocPath = "/latest/dynamic/instance-identity/document"
	AWSPublicIPPath    = "/latest/meta-data/public-ipv4"
	AWSPrivateIPPath   = "/latest/meta-data/local-ipv4"
)

// GCP constants for its metadata service.
const (
	GCPHost           = "http://metadata.google.internal"
	GCPExternalIPPath = "/computeMetadata/v1/instance/network-interfaces/0/access-configs/0/external-ip"
	GCPInternalIPPath = "/computeMetadata/v1/instance/network-interfaces/0/ip"
	GCPProjectIDPath  = "/computeMetadata/v1/project/project-id"
	GCPZonePath       = "/computeMetadata/v1/instance/zone"
)
