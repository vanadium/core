// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The following enables go generate to generate the doc.go file.
//go:generate go run $JIRI_ROOT/release/go/src/v.io/x/lib/cmdline/testdata/gendoc.go . -help

package main

import (
	"fmt"
	"io/ioutil"
	"os/exec"
	"strings"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/security"

	"v.io/x/lib/cmdline"
	"v.io/x/ref/lib/signals"
	"v.io/x/ref/lib/v23cmd"
	_ "v.io/x/ref/runtime/factories/roaming"
	"v.io/x/ref/services/allocator"
	"v.io/x/ref/services/cluster"
)

var (
	nameFlag                string
	serverNameFlag          string
	serverNameRootFlag      string
	serverVersionFlag       string
	deploymentTemplateFlag  string
	globalAdminsFlag        string
	maxInstancesFlag        int
	maxInstancesPerUserFlag int
	diskSizeFlag            string
	gcloudBinFlag           string
	kubectlBinFlag          string
	vkubeBinFlag            string
	vkubeCfgFlag            string
	clusterAgentFlag        string
	blessingSecretFlag      string
	monitoringKeyFileFlag   string

	// HTTP flags.
	assetsFlag              string
	httpAddrFlag            string
	externalURLFlag         string
	oauthCredsFileFlag      string
	secureCookiesFlag       bool
	dashboardGCMMetricFlag  string
	dashboardGCMProjectFlag string
	staticAssetsPrefixFlag  string

	cmdRoot = &cmdline.Command{
		Runner: v23cmd.RunnerFunc(runAllocator),
		Name:   "allocatord",
		Short:  "Runs the allocator service",
		Long:   "Runs the allocator service",
	}
)

const (
	serverNameFlagName     = "server-name"
	serverNameRootFlagName = "server-name-root"
	serverVersionFlagName  = "server-version"
	oauthCredsFileFlagName = "oauth-client-creds-file"
	gcloudFlagName         = "gcloud"
	kubectlFlagName        = "kubectl"
	vkubeFlagName          = "vkube"
)

func main() {
	cmdRoot.Flags.StringVar(&nameFlag, "name", "", "Name to publish for this service.")
	cmdRoot.Flags.StringVar(&serverNameFlag, serverNameFlagName, "", "Name of the servers to allocate. This name is part of the published names in the Vanadium namespace and the names of the Deployments in Kubernetes.")
	cmdRoot.Flags.StringVar(&serverNameRootFlag, serverNameRootFlagName, "", "Namespace root for allocated servers to use when publishing in the Vanadium namespace.  If not set, the namespace root of the allocator server is used.")
	cmdRoot.Flags.StringVar(&serverVersionFlag, serverVersionFlagName, "", "The version of the allocated servers. This value is used in the deployment template expansion.")
	cmdRoot.Flags.StringVar(&deploymentTemplateFlag, "deployment-template", "", "The template for the deployment of the servers to allocate.")
	cmdRoot.Flags.StringVar(&globalAdminsFlag, "global-admins", "", "A comma-separated list of blessing patterns that have access to all the server instances.")
	cmdRoot.Flags.IntVar(&maxInstancesFlag, "max-instances", 10, "The maximum total number of server instances to create.")
	cmdRoot.Flags.IntVar(&maxInstancesPerUserFlag, "max-instances-per-user", 1, "The maximum number of server instances to create per user.")
	cmdRoot.Flags.StringVar(&diskSizeFlag, "server-disk-size", "50GB", "The size of the persistent disk to allocate with the servers.")
	cmdRoot.Flags.StringVar(&gcloudBinFlag, gcloudFlagName, "gcloud", "The gcloud binary to use.")
	cmdRoot.Flags.StringVar(&kubectlBinFlag, kubectlFlagName, "kubectl", "The kubectl binary to use.")
	cmdRoot.Flags.StringVar(&vkubeBinFlag, vkubeFlagName, "vkube", "The vkube binary to use.")
	cmdRoot.Flags.StringVar(&vkubeCfgFlag, "vkube-cfg", "vkube.cfg", "The vkube.cfg to use.")
	cmdRoot.Flags.StringVar(&clusterAgentFlag, "cluster-agent", "", "The address of the cluster-agent.")
	cmdRoot.Flags.StringVar(&blessingSecretFlag, "blessings-secret-file", "", "If set, this file contains the secret to present to the cluster-agent to get the base blessings for the allocated servers.")
	cmdRoot.Flags.StringVar(&assetsFlag, "assets", "", "If set, the directory containing assets (template definitions, css, javascript files etc.) to use in the web interface. If not set, compiled-in assets will be used instead.")
	cmdRoot.Flags.StringVar(&httpAddrFlag, "http-addr", "", "Address on which the HTTP server listens on.  If empty, no HTTP server is started.")
	cmdRoot.Flags.StringVar(&externalURLFlag, "external-url", "", "Public URL for the HTTP server.  Must be specified if --http-addr is specified.")
	cmdRoot.Flags.StringVar(&monitoringKeyFileFlag, "monitoring-key-file", "", "The path to the service account's JSON credentials file.")
	cmdRoot.Flags.StringVar(&oauthCredsFileFlag, oauthCredsFileFlagName, "", "JSON-encoded file containing Google Oauth2 client ID and secret (https://developers.google.com/identity/protocols/OAuth2#basicsteps), as well as the HMAC cookie signing key")
	cmdRoot.Flags.BoolVar(&secureCookiesFlag, "secure-cookies", true, "Whether to use only secure cookies.  Should be true unless running the server without TLS for testing.")
	cmdRoot.Flags.StringVar(&dashboardGCMMetricFlag, "dashboard-gcm-metric", "", "The metric name used to get data from GCM to render dashboard charts.")
	cmdRoot.Flags.StringVar(&dashboardGCMProjectFlag, "dashboard-gcm-project", "", "The project name used to get data from GCM to render dashboard charts.")
	cmdRoot.Flags.StringVar(&staticAssetsPrefixFlag, "static-assets-prefix", "", "Host serving the web assets.")
	cmdline.HideGlobalFlagsExcept()
	cmdline.Main(cmdRoot)
}

func checkFlags(ctx *context.T, env *cmdline.Env) error {
	if nameRoot(ctx) == "" {
		return env.UsageErrorf("--%s not specified, and no default namespace root found", serverNameRootFlagName)
	}
	for f, b := range map[string]string{gcloudFlagName: gcloudBinFlag, kubectlFlagName: kubectlBinFlag, vkubeFlagName: vkubeBinFlag} {
		if _, err := exec.LookPath(b); err != nil {
			return env.UsageErrorf("--%s binary %s not found: %v", f, b, err)
		}
	}
	return nil
}

func runAllocator(ctx *context.T, env *cmdline.Env, args []string) error {
	if err := checkFlags(ctx, env); err != nil {
		return err
	}

	var (
		baseBlessings     security.Blessings
		baseBlessingNames []string
	)
	if clusterAgentFlag == "" || blessingSecretFlag == "" {
		fmt.Fprintln(env.Stderr, "WARNING: Using self-blessed blessings for allocated servers")
		const selfName = "allocator"
		var err error
		if baseBlessings, err = v23.GetPrincipal(ctx).BlessSelf(selfName); err != nil {
			return err
		}
		baseBlessingNames = []string{selfName}
	} else {
		secret, err := ioutil.ReadFile(blessingSecretFlag)
		if err != nil {
			return err
		}
		baseBlessings, err = cluster.ClusterAgentClient(clusterAgentFlag).SeekBlessings(ctx, strings.TrimSpace(string(secret)))
		if err != nil {
			return err
		}
		baseBlessingNames = security.BlessingNames(v23.GetPrincipal(ctx), baseBlessings)
	}

	ctx, server, err := v23.WithNewServer(
		ctx,
		nameFlag,
		allocator.AllocatorServer(&allocatorImpl{baseBlessings, baseBlessingNames}),
		security.AllowEveryone(),
	)
	if err != nil {
		return err
	}
	ah, err := newAssetsHelper(assetsFlag)
	if err != nil {
		return err
	}
	ctx.Infof("Listening on: %v", server.Status().Endpoints)
	if httpAddrFlag != "" {
		if oauthCredsFileFlag == "" {
			return env.UsageErrorf("--%s must be provided", oauthCredsFileFlagName)
		}
		oauthCreds, err := clientCredsFromFile(oauthCredsFileFlag)
		if err != nil {
			return err
		}
		cleanup := startHTTP(ctx, httpArgs{
			addr:                httpAddrFlag,
			externalURL:         externalURLFlag,
			dashboardGCMMetric:  dashboardGCMMetricFlag,
			dashboardGCMProject: dashboardGCMProjectFlag,
			monitoringKeyFile:   monitoringKeyFileFlag,
			oauthCreds:          oauthCreds,
			serverName:          serverNameFlag,
			secureCookies:       secureCookiesFlag,
			baseBlessings:       baseBlessings,
			baseBlessingNames:   baseBlessingNames,
			staticAssetsPrefix:  staticAssetsPrefixFlag,
			assets:              ah,
		})
		defer cleanup()
	}
	<-signals.ShutdownOnSignals(ctx)
	return nil
}
