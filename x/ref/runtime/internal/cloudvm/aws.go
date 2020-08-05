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
	onceAWS sync.Once
	onAWS   bool
)

// OnAWS returns true if this process is running on Amazon Web Services.
// If true, the the stats variables AWSAccountIDStatName and GCPRegionStatName
// are set.
func OnAWS(ctx context.Context, timeout time.Duration) bool {
	onceAWS.Do(func() {
		onAWS = awsInit(ctx, timeout)
	})
	return onAWS
}

// AWSPublicAddrs returns the current public IP of this AWS instance.
func AWSPublicAddrs(ctx context.Context, timeout time.Duration) ([]net.Addr, error) {
	return awsGetAddr(ctx, awsExternalURL(), timeout)
}

// AWSPrivateAddrs returns the current private Addrs of this AWS instance.
func AWSPrivateAddrs(ctx context.Context, timeout time.Duration) ([]net.Addr, error) {
	return awsGetAddr(ctx, awsInternalURL(), timeout)
}

func awsGet(ctx context.Context, url string, timeout time.Duration) ([]byte, error) {
	client := &http.Client{Timeout: timeout}
	token, err := awsSetIMDSv2Token(ctx, awsTokenURL(), timeout)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Add("X-aws-ec2-metadata-token", token)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, err
	}
	if server := resp.Header["Server"]; len(server) != 1 || server[0] != "EC2ws" {
		return nil, fmt.Errorf("wrong headers")
	}
	return ioutil.ReadAll(resp.Body)
}

// awsInit returns true if it can access AWS project metadata. It also
// creates two stats variables with the account ID and zone.
func awsInit(ctx context.Context, timeout time.Duration) bool {
	body, err := awsGet(ctx, awsIdentityDocURL(), timeout)
	if err != nil {
		return false
	}
	doc := map[string]interface{}{}
	if err := json.Unmarshal(body, &doc); err != nil {
		return false
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
	return found == 2
}

func awsGetAddr(ctx context.Context, url string, timeout time.Duration) ([]net.Addr, error) {
	body, err := awsGet(ctx, url, timeout)
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
