// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package vxray integrates amazon's xray distributed tracing system
// with vanadium's vtrace. When used this implementation of vtrace
// will publish to xray in addition to the normal vtrace functionality.
package vxray

import (
	"fmt"

	"github.com/aws/aws-xray-sdk-go/awsplugins/beanstalk"
	"github.com/aws/aws-xray-sdk-go/awsplugins/ec2"
	"github.com/aws/aws-xray-sdk-go/awsplugins/ecs"
	"github.com/aws/aws-xray-sdk-go/xray"
	"github.com/aws/aws-xray-sdk-go/xraylog"
	"v.io/v23/context"
	"v.io/v23/logging"
	"v.io/v23/vtrace"
	"v.io/x/ref/lib/flags"
	libvtrace "v.io/x/ref/lib/vtrace"
)

type options struct {
	mergeLogging  bool
	mapToHTTP     bool
	newStore      bool
	newStoreFlags flags.VtraceFlags
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

func initXRay(ctx *context.T, config xray.Config, opts []Option) (*context.T, *options, error) {
	o := &options{mapToHTTP: true}
	for _, fn := range opts {
		fn(o)
	}
	if err := xray.Configure(config); err != nil {
		ctx.Errorf("failed to configure xray context: %v", err)
		return ctx, nil, err
	}
	if o.mergeLogging {
		xray.SetLogger(&xraylogger{context.LoggerFromContext(ctx)})
	}
	ctx, err := WithConfig(ctx, config)
	return ctx, o, err
}

// InitXRay configures the AWS xray service and returns a context containing
// the xray configuration. This should only be called once. The vflags argument
// is used solely to check if xray tracing is enabled and not to create a
// new vtrace.Store, if a new store is required, the
func InitXRay(ctx *context.T, vflags flags.VtraceFlags, config xray.Config, opts ...Option) (*context.T, error) {
	if !vflags.EnableAWSXRay {
		return ctx, nil
	}
	octx := ctx
	ctx, options, err := initXRay(ctx, config, opts)
	if err != nil {
		return octx, err
	}

	if options.newStore {
		store, err := libvtrace.NewStore(options.newStoreFlags)
		if err != nil {
			return octx, err
		}
		ctx = vtrace.WithStore(ctx, store)
	}
	mgr := &manager{mapToHTTP: options.mapToHTTP}
	ctx = vtrace.WithManager(ctx, mgr)
	return ctx, nil
}
