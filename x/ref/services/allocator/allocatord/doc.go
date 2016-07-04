// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated via go generate.
// DO NOT UPDATE MANUALLY

/*
Runs the allocator service

Usage:
   allocatord [flags]

The allocatord flags are:
 -assets=
   If set, the directory containing assets (template definitions, css,
   javascript files etc.) to use in the web interface. If not set, compiled-in
   assets will be used instead.
 -blessings-secret-file=
   If set, this file contains the secret to present to the cluster-agent to get
   the base blessings for the allocated servers.
 -cluster-agent=
   The address of the cluster-agent.
 -dashboard-gcm-metric=
   The metric name used to get data from GCM to render dashboard charts.
 -dashboard-gcm-project=
   The project name used to get data from GCM to render dashboard charts.
 -deployment-template=
   The template for the deployment of the servers to allocate.
 -external-url=
   Public URL for the HTTP server.  Must be specified if --http-addr is
   specified.
 -gcloud=gcloud
   The gcloud binary to use.
 -global-admins=
   A comma-separated list of blessing patterns that have access to all the
   server instances.
 -http-addr=
   Address on which the HTTP server listens on.  If empty, no HTTP server is
   started.
 -kubectl=kubectl
   The kubectl binary to use.
 -max-instances=10
   The maximum total number of server instances to create.
 -max-instances-per-user=1
   The maximum number of server instances to create per user.
 -monitoring-key-file=
   The path to the service account's JSON credentials file.
 -name=
   Name to publish for this service.
 -oauth-client-creds-file=
   JSON-encoded file containing Google Oauth2 client ID and secret
   (https://developers.google.com/identity/protocols/OAuth2#basicsteps), as well
   as the HMAC cookie signing key
 -secure-cookies=true
   Whether to use only secure cookies.  Should be true unless running the server
   without TLS for testing.
 -server-disk-size=50GB
   The size of the persistent disk to allocate with the servers.
 -server-name=
   Name of the servers to allocate. This name is part of the published names in
   the Vanadium namespace and the names of the Deployments in Kubernetes.
 -server-name-root=
   Namespace root for allocated servers to use when publishing in the Vanadium
   namespace.  If not set, the namespace root of the allocator server is used.
 -server-version=
   The version of the allocated servers. This value is used in the deployment
   template expansion.
 -static-assets-prefix=
   Host serving the web assets.
 -vkube=vkube
   The vkube binary to use.
 -vkube-cfg=vkube.cfg
   The vkube.cfg to use.

The global flags are:
 -alsologtostderr=true
   log to standard error as well as files
 -log_backtrace_at=:0
   when logging hits line file:N, emit a stack trace
 -log_dir=
   if non-empty, write log files to this directory
 -logtostderr=false
   log to standard error instead of files
 -max_stack_buf_size=4292608
   max size in bytes of the buffer to use for logging stack traces
 -metadata=<just specify -metadata to activate>
   Displays metadata for the program and exits.
 -stderrthreshold=2
   logs at or above this threshold go to stderr
 -time=false
   Dump timing information to stderr before exiting the program.
 -v=0
   log level for V logs
 -v23.credentials=
   directory to use for storing security credentials
 -v23.i18n-catalogue=
   18n catalogue files to load, comma separated
 -v23.namespace.root=[/(dev.v.io:r:vprod:service:mounttabled)@ns.dev.v.io:8101]
   local namespace root; can be repeated to provided multiple roots
 -v23.permissions.file=map[]
   specify a perms file as <name>:<permsfile>
 -v23.permissions.literal=
   explicitly specify the runtime perms as a JSON-encoded access.Permissions.
   Overrides all --v23.permissions.file flags.
 -v23.proxy=
   object name of proxy service to use to export services across network
   boundaries
 -v23.tcp.address=
   address to listen on
 -v23.tcp.protocol=wsh
   protocol to listen with
 -v23.vtrace.cache-size=1024
   The number of vtrace traces to store in memory.
 -v23.vtrace.collect-regexp=
   Spans and annotations that match this regular expression will trigger trace
   collection.
 -v23.vtrace.dump-on-shutdown=true
   If true, dump all stored traces on runtime shutdown.
 -v23.vtrace.sample-rate=0
   Rate (from 0.0 to 1.0) to sample vtrace traces.
 -v23.vtrace.v=0
   The verbosity level of the log messages to be captured in traces
 -vmodule=
   comma-separated list of globpattern=N settings for filename-filtered logging
   (without the .go suffix).  E.g. foo/bar/baz.go is matched by patterns baz or
   *az or b* but not by bar/baz or baz.go or az or b.*
 -vpath=
   comma-separated list of regexppattern=N settings for file pathname-filtered
   logging (without the .go suffix).  E.g. foo/bar/baz.go is matched by patterns
   foo/bar/baz or fo.*az or oo/ba or b.z but not by foo/bar/baz.go or fo*az
*/
package main
