// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package factory

import (
	"errors"
	"fmt"
	"os"
	"sync"

	"v.io/v23/context"
	"v.io/v23/discovery"

	idiscovery "v.io/x/ref/lib/discovery"
)

type (
	pluginFactory    func(ctx *context.T, host string) (idiscovery.Plugin, error)
	pluginFactoryMap map[string]pluginFactory
)

var (
	pluginFactories pluginFactoryMap
)

// SetPluginFactory sets the plugin factory with the given name.
// This should be called before v23.NewDiscovery() is called.
func SetPluginFactory(name string, factory pluginFactory) {
	pluginFactories[name] = factory
}

type lazyFactory struct {
	host      string
	protocols []string

	once sync.Once
	f    idiscovery.Factory
	err  error
}

func (l *lazyFactory) New(ctx *context.T) (discovery.T, error) {
	l.once.Do(func() { l.f, l.err = newFactory(ctx, l.host, l.protocols) })
	if l.err != nil {
		return nil, l.err
	}
	return l.f.New(ctx)
}

func (l *lazyFactory) Shutdown() {
	l.once.Do(func() { l.err = errors.New("factory closed") })
	if l.f != nil {
		l.f.Shutdown()
	}
}

// New returns a new discovery factory with the given protocols.
//
// We instantiate a factory lazily so that we do not turn it on until
// it is actually used.
func New(_ *context.T, protocols ...string) (idiscovery.Factory, error) {
	host, _ := os.Hostname()
	if len(host) == 0 {
		// TODO(jhahn): Should we handle error here?
		host = "v23"
	}

	if len(protocols) == 0 {
		// Use all registered protocols.
		for p := range pluginFactories {
			protocols = append(protocols, p)
		}
	}
	// Verify protocols.
	for _, p := range protocols {
		if _, exists := pluginFactories[p]; !exists {
			return nil, fmt.Errorf("discovery protocol %q is not supported", p)
		}
	}

	return &lazyFactory{host: host, protocols: protocols}, nil
}

func newFactory(ctx *context.T, host string, protocols []string) (idiscovery.Factory, error) {
	if injectedFactory != nil {
		return injectedFactory, nil
	}

	plugins := make([]idiscovery.Plugin, 0, len(protocols))
	for _, p := range protocols {
		factory := pluginFactories[p]
		plugin, err := factory(ctx, host)
		if err != nil {
			return nil, err
		}
		plugins = append(plugins, plugin)
	}
	return idiscovery.NewFactory(ctx, plugins...)
}

var injectedFactory idiscovery.Factory

// InjectFactory allows a runtime to use the given discovery factory. This
// should be called before v23.NewDiscovery() is called. Mostly used for testing.
func InjectFactory(factory idiscovery.Factory) {
	injectedFactory = factory
}
