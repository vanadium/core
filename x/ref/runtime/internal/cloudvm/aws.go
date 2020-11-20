// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package cloudvm provides functions to test whether the current process is
// running on Google Compute Engine or Amazon Web Services, and to extract
// settings from this environment.
package cloudvm

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"sync"
	"time"

	"v.io/v23/logging"
	"v.io/x/ref/lib/stats"
	"v.io/x/ref/runtime/internal/cloudvm/cloudpaths"
)

var awsHost string = cloudpaths.AWSHost

// SetAWSMetadataHost can be used to override the default metadata host
// for testing purposes.
func SetAWSMetadataHost(host string) {
	awsHost = host
}

func awsMetadataHost() string {
	return awsHost
}

func awsExternalURL() string {
	return awsMetadataHost() + cloudpaths.AWSPublicIPPath
}
func awsInternalURL() string {
	return awsMetadataHost() + cloudpaths.AWSPrivateIPPath
}
func awsIdentityDocURL() string {
	return awsMetadataHost() + cloudpaths.AWSIdentityDocPath
}
func awsTokenURL() string {
	return awsMetadataHost() + cloudpaths.AWSTokenPath
}

const (
	// AWSAccountIDStatName is the name of a v.io/x/ref/lib/stats
	// string variable containing the account id.
	AWSAccountIDStatName = "system/aws/account-id"
	// AWSRegionStatName is the name of a v.io/x/ref/lib/stats
	// string variable containing the region.
	AWSRegionStatName = "system/aws/zone"
)

var (
	onceAWS  sync.Once
	onAWS    bool
	onIMDSv2 bool
)

// OnAWS returns true if this process is running on Amazon Web Services.
// If true, the the stats variables AWSAccountIDStatName and GCPRegionStatName
// are set.
func OnAWS(ctx context.Context, logger logging.Logger, timeout time.Duration) bool {
	onceAWS.Do(func() {
		onAWS, onIMDSv2 = awsInit(ctx, logger, timeout)
		logger.VI(1).Infof("OnAWS: onAWS: %v, onIMDSv2: %v", onAWS, onIMDSv2)
	})
	return onAWS
}

// AWSPublicAddrs returns the current public IP of this AWS instance.
// Must be called after OnAWS.
func AWSPublicAddrs(ctx context.Context, timeout time.Duration) ([]net.Addr, error) {
	return awsGetAddr(ctx, onIMDSv2, awsExternalURL(), timeout)
}

// AWSPrivateAddrs returns the current private Addrs of this AWS instance.
// Must be called after OnAWS.
func AWSPrivateAddrs(ctx context.Context, timeout time.Duration) ([]net.Addr, error) {
	return awsGetAddr(ctx, onIMDSv2, awsInternalURL(), timeout)
}

func awsGet(ctx context.Context, imdsv2 bool, url string, timeout time.Duration) ([]byte, error) {
	client := &http.Client{Timeout: timeout}
	var token string
	var err error
	if imdsv2 {
		token, err = awsSetIMDSv2Token(ctx, awsTokenURL(), timeout)
		if err != nil {
			return nil, err
		}
	}
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	if len(token) > 0 {
		req.Header.Add("X-aws-ec2-metadata-token", token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP Error: %v %v", url, resp.StatusCode)
	}
	if server := resp.Header["Server"]; len(server) != 1 || server[0] != "EC2ws" {
		return nil, fmt.Errorf("wrong headers")
	}
	return ioutil.ReadAll(resp.Body)
}

// awsInit returns true if it can access AWS project metadata and the version
// of the metadata service it was able to access. It also
// creates two stats variables with the account ID and zone.
func awsInit(ctx context.Context, logger logging.Logger, timeout time.Duration) (bool, bool) {
	v2 := false
	// Try the v1 service first since it should always work unless v2
	// is specifically configured (and hence v1 is disabled), in which
	// case the expectation is that it fails fast with a 4xx HTTP error.
	body, err := awsGet(ctx, false, awsIdentityDocURL(), timeout)
	if err != nil {
		logger.VI(1).Infof("failed to access v1 metadata service: %v", err)
		// can't access v1, try v2.
		body, err = awsGet(ctx, true, awsIdentityDocURL(), timeout)
		if err != nil {
			logger.VI(1).Infof("failed to access v2 metadata service: %v", err)
			return false, false
		}
		v2 = true
	}
	doc := map[string]interface{}{}
	if err := json.Unmarshal(body, &doc); err != nil {
		logger.VI(1).Infof("failed to unmarshal metadata service response: %s: %v", body, err)
		return false, false
	}
	found := 0
	for _, v := range []struct {
		name, key string
	}{
		{AWSAccountIDStatName, "accountId"},
		{AWSRegionStatName, "region"},
	} {
		if _, present := doc[v.key]; present {
			if val, ok := doc[v.key].(string); ok {
				found++
				stats.NewString(v.name).Set(val)
			}
		}
	}
	return found == 2, v2
}

func awsGetAddr(ctx context.Context, imdsv2 bool, url string, timeout time.Duration) ([]net.Addr, error) {
	body, err := awsGet(ctx, imdsv2, url, timeout)
	if err != nil {
		return nil, err
	}
	ip := net.ParseIP(string(body))
	if len(ip) == 0 {
		return nil, nil
	}
	return []net.Addr{&net.IPAddr{IP: ip}}, nil
}

func awsSetIMDSv2Token(ctx context.Context, url string, timeout time.Duration) (string, error) {
	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequestWithContext(ctx, "PUT", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Add("X-aws-ec2-metadata-token-ttl-seconds", "60")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", err
	}
	token, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(token), nil
}
