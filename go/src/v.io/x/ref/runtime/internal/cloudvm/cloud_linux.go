// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build linux,!android

// Package cloudvm provides functions to test whether the current process is
// running on Google Compute Engine or Amazon Web Services, and to extract
// settings from this environment.
package cloudvm

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"sync"
	"time"

	"v.io/x/ref/lib/stats"
)

// This URL returns the external IP address assigned to the local GCE instance.
// If a HTTP GET request fails for any reason, this is not a GCE instance. If
// the result of the GET request doesn't contain a "Metadata-Flavor: Google"
// header, it is also not a GCE instance. The body of the document contains the
// external IP address, if present. Otherwise, the body is empty.
// See https://developers.google.com/compute/docs/metadata for details.
const gceExternalUrl = "http://metadata.google.internal/computeMetadata/v1/instance/network-interfaces/0/access-configs/0/external-ip"
const gceInternalUrl = "http://metadata.google.internal/computeMetadata/v1/instance/network-interfaces/0/ip"
const awsExternalUrl = "http://169.254.169.254/latest/meta-data/public-ipv4"
const awsInternalUrl = "http://169.254.169.254/latest/meta-data/local-ipv4"

var (
	onceGCE    sync.Once
	onceAWS    sync.Once
	onGCE      bool
	onAWS      bool
	internalIP net.IP
	externalIP net.IP
)

func InitGCE(timeout time.Duration, cancel <-chan struct{}) {
	onceGCE.Do(func() {
		if onAWS {
			return
		}
		gceTest(timeout, cancel)
	})
}

func InitAWS(timeout time.Duration, cancel <-chan struct{}) {
	onceAWS.Do(func() {
		if onGCE {
			return
		}
		awsTest(timeout, cancel)
	})
}

func RunningOnGCE() bool {
	return onGCE
}

func RunningOnAWS() bool {
	return onAWS
}

// InternalIPAddress returns the internal IP address of this Google Compute
// Engine or AWS instance, or nil if there is none. Must be called after
// InitGCE / InitAWS.
func InternalIPAddress() net.IP {
	return internalIP
}

// ExternalIPAddress returns the external IP address of this Google Compute
// Engine or AWS instance, or nil if there is none. Must be called after
// InitGCE / InitAWS.
func ExternalIPAddress() net.IP {
	return externalIP
}

func gceTest(timeout time.Duration, cancel <-chan struct{}) {
	var err error
	internalIP, _ = gceGetIP(gceInternalUrl, timeout, cancel)
	if externalIP, err = gceGetIP(gceExternalUrl, timeout, cancel); err != nil {
		return
	}

	vars := []struct {
		name, url string
	}{
		{"system/gce/project-id", "http://metadata.google.internal/computeMetadata/v1/project/project-id"},
		{"system/gce/zone", "http://metadata.google.internal/computeMetadata/v1/instance/zone"},
	}
	for _, v := range vars {
		body, err := gceGetMeta(v.url, timeout, cancel)
		if err != nil || body == "" {
			return
		}
		stats.NewString(v.name).Set(body)
	}
	onGCE = true
}

func gceGetIP(url string, timeout time.Duration, cancel <-chan struct{}) (net.IP, error) {
	body, err := gceGetMeta(url, timeout, cancel)
	if err != nil {
		return nil, err
	}
	return net.ParseIP(body), nil
}

func gceGetMeta(url string, timeout time.Duration, cancel <-chan struct{}) (string, error) {
	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Cancel = cancel
	req.Header.Add("Metadata-Flavor", "Google")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("http error: %d", resp.StatusCode)
	}
	if flavor := resp.Header["Metadata-Flavor"]; len(flavor) != 1 || flavor[0] != "Google" {
		return "", fmt.Errorf("unexpected http header: %q", flavor)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func awsTest(timeout time.Duration, cancel <-chan struct{}) {
	externalIP = awsGetIP(awsExternalUrl, timeout, cancel)
	internalIP = awsGetIP(awsInternalUrl, timeout, cancel)
	onAWS = true
}

func awsGetIP(url string, timeout time.Duration, cancel <-chan struct{}) (net.IP) {
	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil
	}
	req.Cancel = cancel
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil
	}
	if server := resp.Header["Server"]; len(server) != 1 || server[0] != "EC2ws" {
		return nil
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil
	}
	return net.ParseIP(string(body))
}