// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package vxray integrates amazon's xray distributed tracing system
// with vanadium's vtrace. When used this implementation of vtrace
// will publish to xray in addition to the normal vtrace functionality.
package vxray

import (
	"fmt"
	"os"

	"github.com/aws/aws-xray-sdk-go/awsplugins/beanstalk"
	"github.com/aws/aws-xray-sdk-go/awsplugins/ec2"
	"github.com/aws/aws-xray-sdk-go/awsplugins/ecs"
	"github.com/aws/aws-xray-sdk-go/xray"
	"github.com/aws/aws-xray-sdk-go/xraylog"
	"v.io/v23/context"
	"v.io/v23/logging"
	"v.io/v23/vtrace"
	"v.io/x/ref/lib/aws/vxray/internal"
	"v.io/x/ref/lib/flags"
	libvtrace "v.io/x/ref/lib/vtrace"
)

type options struct {
	mergeLogging            bool
	mapToHTTP               bool
	newStore                bool
	newStoreFlags           flags.VtraceFlags
	configMap, configMapKey string
	containerized           bool
}

// Option represents an option to InitXRay.
type Option func(o *options)

// EC2Plugin initializes the EC2 plugin.
func EC2Plugin() Option {
	return func(o *options) {
		ec2.Init()
	}
}

// ECSPlugin initializes the ECS plugin.
func ECSPlugin() Option {
	return func(o *options) {
		ecs.Init()
	}
}

// BeanstalkPlugin initializes the BeanstalkPlugin plugin.
func BeanstalkPlugin() Option {
	return func(o *options) {
		beanstalk.Init()
	}
}

// KubernetesCluster configures obtaining information about the process'
// current environment when running under Kubernetes (k8s), whether managed by
// AWS EKS or any other control plane implementation. It requires that the
// K8S configuration creates a configmap that contains the cluster name.
// The configMap argument names that configmap and configMapKey
// is the key in that configmap for the cluster name. For example, when using
// the AWS cloudwatch/insights/xray-daemon daemonset the values for those
// would be:
//     /api/v1/namespaces/amazon-cloudwatch/configmaps/cluster-info
//     cluster.name
//
// When configured, xray segments will contain a 'cluster_name' annotation.
func KubernetesCluster(configMap, configMapKey string) Option {
	return func(o *options) {
		o.configMap, o.configMapKey = configMap, configMapKey
	}
}

// EKSCluster calls KubernetesCluster with the values commonly used
// with EKS clusters.
func EKSCluster() Option {
	return KubernetesCluster("/api/v1/namespaces/amazon-cloudwatch/configmaps/cluster-info", "cluster.name")
}

// ContainerIDAndHost requests that container id and host information be
// obtained and added to traces. The container id is obtained by parsing
// the /proc/self/cgroup file, and the host by call the operating system's
// hostname function. When running under kubernetes for example, the
//
// When configured, xray segments will contain 'container_id' and 'container_host'
// annotations.
func ContainerIDAndHost() Option {
	return func(o *options) {
		o.containerized = true
	}
}

// MergeLogging arrays for xray logging messages to be merged with vanadium
// log messages.
func MergeLogging(v bool) Option {
	return func(o *options) {
		o.mergeLogging = v
	}
}

// WithNewStore requests that a new vtrace.Store be created and stored
// in the context returned by InitXRay.
func WithNewStore(vflags flags.VtraceFlags) Option {
	return func(o *options) {
		o.newStore = true
		o.newStoreFlags = vflags
	}
}

// MapToHTTP maps vtrace fields/concepts to xray's http-centric xray.RequestData
// fields. This relies on knowledge of how the vanadium runtime
// encodes metadata in a span. In particular, it assumes that
// metadata values are set for:
//    - name   string // the vanadium name for the service, maps to a url
//    - method string // the vanadium method, maps to an http method,
//                       this is informational only since xray has limits on
//                       the length of a method name.
//    - clientAddr string // the client invoking the rpc, maps to clientip.
//    The user agent is set to vanadium.
// NOTE that this mapping will only occur if there is no existing xray.RequestData
// encoded in the segment. This allows for xray segments created directly by
// xray API calls prior to spans being created to be supported without losing
// any of the original HTTP request data encoded therein.
func MapToHTTP(v bool) Option {
	return func(o *options) {
		o.mapToHTTP = v
	}
}

type xraylogger struct {
	logging.Logger
}

func (xl *xraylogger) Log(level xraylog.LogLevel, msg fmt.Stringer) {
	switch level {
	case xraylog.LogLevelInfo, xraylog.LogLevelWarn:
		xl.Info(msg.String())
	case xraylog.LogLevelError:
		xl.Error(msg.String())
	case xraylog.LogLevelDebug:
		xl.VI(1).Info(msg.String())
	}
}

func (m *manager) initXRay(ctx *context.T, config xray.Config) (*context.T, error) {
	if err := xray.Configure(config); err != nil {
		ctx.Errorf("failed to configure xray context: %v", err)
		return ctx, err
	}
	if m.options.mergeLogging {
		xray.SetLogger(&xraylogger{context.LoggerFromContext(ctx)})
	}
	ctx, err := WithConfig(ctx, config)
	return ctx, err
}

// InitXRay configures the AWS xray service and returns a context containing
// the xray configuration. This should only be called once. The vflags argument
// is used solely to check if xray tracing is enabled and not to create a
// new vtrace.Store, if a new/alternate store is required, the WithNewStore option
// should be used to specify the store to be used.
func InitXRay(ctx *context.T, vflags flags.VtraceFlags, config xray.Config, opts ...Option) (*context.T, error) {
	if !vflags.EnableAWSXRay {
		return ctx, nil
	}
	octx := ctx
	mgr := &manager{}
	mgr.options.mapToHTTP = true
	for _, fn := range opts {
		fn(&mgr.options)
	}
	ctx, err := mgr.initXRay(ctx, config)
	if err != nil {
		return octx, err
	}
	if mgr.options.newStore {
		store, err := libvtrace.NewStore(mgr.options.newStoreFlags)
		if err != nil {
			return octx, err
		}
		ctx = vtrace.WithStore(ctx, store)
	}
	if mgr.options.containerized {
		if hostNameErr == nil {
			mgr.containerHost = hostName
		} else {
			ctx.Infof("failed to obtain host name from: %v", hostNameErr)
		}
		cgroupFile := "/proc/self/cgroup"
		if cid, err := internal.GetContainerID(cgroupFile); err == nil {
			mgr.containerID = cid
		} else {
			ctx.Infof("failed to obtain container id", err)
		}
	}
	if cm := mgr.options.configMap; len(cm) > 0 {
		if clusterName, err := internal.GetEKSClusterName(ctx, cm, mgr.options.configMapKey); err == nil {
			mgr.clusterName = clusterName
		} else {
			ctx.Infof("failed to obtain cluster name from %v.%v: %v", cm, mgr.options.configMapKey, err)
		}
	}
	ctx = vtrace.WithManager(ctx, mgr)
	return ctx, nil
}

var (
	hostName    string
	hostNameErr error
)

func init() {
	hostName, hostNameErr = os.Hostname()
}
