// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package cloudvm provides functions to test whether the current process is
// running on Google Compute Engine or Amazon Web Services, and to extract
// settings from this environment.
package cloudvm

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"sync"
	"time"

	"v.io/x/ref/lib/stats"
	"v.io/x/ref/runtime/internal/cloudvm/cloudpaths"
)

// This URL returns the external IP address assigned to the local GCE instance.
// If a HTTP GET request fails for any reason, this is not a GCE instance. If
// the result of the GET request doesn't contain a "Metadata-Flavor: Google"
// header, it is also not a GCE instance. The body of the document contains the
// external IP address, if present. Otherwise, the body is empty.
// See https://developers.google.com/compute/docs/metadata for details.
// They are variables so that tests may override them.

var gcpHost string = cloudpaths.GCPHost

// SetGCPMetadataHost can be used to override the default metadata host
// for testing purposes.
func SetGCPMetadataHost(host string) {
	gcpHost = host
}

func gcpMetadataHost() string {
	return gcpHost
}

func gcpExternalURL() string {
	return gcpMetadataHost() + cloudpaths.GCPExternalIPPath
}
func gcpInternalURL() string {
	return gcpMetadataHost() + cloudpaths.GCPInternalIPPath
}
func gcpProjectIDURL() string {
	return gcpMetadataHost() + cloudpaths.GCPProjectIDPath
}
func gcpZoneIDUrl() string {
	return gcpMetadataHost() + cloudpaths.GCPZonePath
}

const (

	// GCPProjectIDStatName is the name of a v.io/x/ref/lib/stats
	// string variable containing the project id.
	GCPProjectIDStatName = "system/gcp/project-id"
	// GCPRegionStatName is the name of a v.io/x/ref/lib/stats
	// string variable containing the region.
	GCPRegionStatName = "system/gcp/zone"
)

var (
	onceGCP sync.Once
	onGCP   bool
)

// OnGCP returns true if this process is running on Google Compute Platform.
// If true, the the stats variables GCPProjectIDStatName and GCPRegionStatName
// are set.
func OnGCP(ctx context.Context, timeout time.Duration) bool {
	onceGCP.Do(func() {
		onGCP = gcpInit(ctx, timeout)
	})
	return onGCP
}

// gcpInit returns true if it can access GCP project metadata. It also
// creates two stats variables with the project ID and zone.
func gcpInit(ctx context.Context, timeout time.Duration) bool {
	vars := []struct {
		name, url string
	}{
		{GCPProjectIDStatName, gcpProjectIDURL()},
		{GCPRegionStatName, gcpZoneIDUrl()},
	}
	for _, v := range vars {
		body, err := gcpGetMeta(ctx, v.url, timeout)
		if err != nil || body == "" {
			return false
		}
		stats.NewString(v.name).Set(body)
	}
	return true
}

// GCPPublicAddrs returns the current public addresses of this GCP instance.
func GCPPublicAddrs(ctx context.Context, timeout time.Duration) ([]net.Addr, error) {
	return gcpGetAddr(ctx, gcpExternalURL(), timeout)
}

// GCPPrivateAddrs returns the current private addresses of this GCP instance.
func GCPPrivateAddrs(ctx context.Context, timeout time.Duration) ([]net.Addr, error) {
	return gcpGetAddr(ctx, gcpInternalURL(), timeout)
}

func gcpGetAddr(ctx context.Context, url string, timeout time.Duration) ([]net.Addr, error) {
	body, err := gcpGetMeta(ctx, url, timeout)
	if err != nil {
		return nil, err
	}
	ip := net.ParseIP(body)
	if len(ip) == 0 {
		return nil, nil
	}
	return []net.Addr{&net.IPAddr{IP: ip}}, nil
}

func gcpGetMeta(ctx context.Context, url string, timeout time.Duration) (string, error) {
	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}
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
