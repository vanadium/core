// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"v.io/v23/logging"
	"v.io/x/lib/netstate"
	"v.io/x/ref/lib/flags"
	"v.io/x/ref/runtime/internal/cloudvm"
)

type getAddrFunc func(context.Context, time.Duration) ([]net.Addr, error)

type CloudVM struct {
	cfg                           *flags.VirtualizedFlags
	logger                        logging.Logger
	dnsNameAndPort                string
	includePrivateAddresses       bool
	mu                            sync.Mutex
	getPublicAddr, getPrivateAddr getAddrFunc // GUARDED_BY(mu)
	addrs                         []net.Addr  // GUARDED_BY(mu)
}

var (
	once   sync.Once
	cvm    *CloudVM
	cvmErr error
)

// InitCloudVM initializes the CloudVM metadata using the configuration
// provided by VirtualizedFlags. It implements rpc.AddressChooser and
// provides a RefreshAddress method to update addresses based on current metadata
// settings. If PublicAddress is set it is used, then PublicDNSName and finally,
// if a VirtualizationProvider is defined its metadata service will be used.
// If the AdvertisePrivateAddresses flag is set then the private addresses
// and the candidate addresses supplied to the address chooser will also be
// returned by ChoseAddresses, otherwise only the public/external ones
// will be returned.
func InitCloudVM(ctx context.Context, logger logging.Logger, fl *flags.VirtualizedFlags) (*CloudVM, error) {
	once.Do(func() {
		cvm, cvmErr = newCloudVM(ctx, logger, fl)
	})
	return cvm, cvmErr
}

func newCloudVM(ctx context.Context, logger logging.Logger, fl *flags.VirtualizedFlags) (*CloudVM, error) {
	cvm := &CloudVM{
		cfg:                     fl,
		logger:                  logger,
		includePrivateAddresses: fl.AdvertisePrivateAddresses,
	}

	if len(fl.PublicDNSName) > 0 {
		if len(fl.PublicDNSPort) == 0 || fl.PublicDNSPort == "0" {
			cvm.dnsNameAndPort = fl.PublicDNSName
		} else {
			cvm.dnsNameAndPort = net.JoinHostPort(fl.PublicDNSName, fl.PublicDNSPort)
		}
	}

	isaws := func() {
		cvm.getPublicAddr = cloudvm.AWSPublicAddrs
		cvm.getPrivateAddr = cloudvm.AWSPrivateAddrs
	}
	isgcp := func() {
		cvm.getPublicAddr = cloudvm.GCPPublicAddrs
		cvm.getPrivateAddr = cloudvm.GCPPrivateAddrs
	}

	refresh := func() (*CloudVM, error) {
		if err := cvm.RefreshAddresses(ctx); err != nil {
			return nil, err
		}
		return cvm, nil
	}

	switch fl.VirtualizationProvider {
	case flags.AWS:
		if !cloudvm.OnAWS(ctx, time.Second) {
			return nil, fmt.Errorf("this process is not running on AWS even though its command line says it is")
		}
		isaws()
		return refresh()
	case flags.GCP:
		if !cloudvm.OnGCP(ctx, time.Second) {
			return nil, fmt.Errorf("this process is not running on GCP even though its command line says it is")
		}
		isgcp()
		return refresh()
	}
	noop := func(context.Context, time.Duration) ([]net.Addr, error) {
		return nil, nil
	}
	cvm.getPublicAddr, cvm.getPrivateAddr = noop, noop
	return refresh()
}

// RefreshAddresses updates the addresses from the viurtualization/cloud
// providers metadata if appropriate to do so.
func (cvm *CloudVM) RefreshAddresses(ctx context.Context) error {
	cvm.mu.Lock()
	defer cvm.mu.Unlock()

	firstAddress := func(addrs []net.Addr) string {
		if len(addrs) == 0 {
			return ""
		}
		return addrs[0].String()
	}

	switch {
	case len(cvm.cfg.PublicAddress.String()) > 0:
		cvm.addrs = nil
		if len(cvm.cfg.PublicAddress.IP) > 1 {
			for _, a := range cvm.cfg.PublicAddress.IP {
				p := a.Network()
				if p == "ip" {
					p = cvm.cfg.PublicProtocol.Protocol
				}
				na := netstate.NewNetAddr(
					p,
					net.JoinHostPort(a.String(), cvm.cfg.PublicAddress.Port))
				cvm.addrs = append(cvm.addrs, na)
			}
		} else {
			cvm.addrs = []net.Addr{netstate.NewNetAddr(
				cvm.cfg.PublicProtocol.Protocol,
				cvm.cfg.PublicAddress.Address)}
		}
		cvm.logger.Infof("cloudvm.RefreshAddresses: using public addresses, first one is: %v...", firstAddress(cvm.addrs))
		return nil
	case len(cvm.cfg.PublicDNSName) > 0:
		cvm.addrs = []net.Addr{netstate.NewNetAddr(
			cvm.cfg.PublicProtocol.Protocol,
			cvm.dnsNameAndPort)}
		cvm.logger.Infof("cloudvm.RefreshAddresses: using dnsname: %v", cvm.cfg.PublicDNSName)
		return nil
	}
	var err error
	cvm.addrs, err = cvm.getPublicAddr(ctx, time.Second)
	if err != nil {
		return err
	}
	cvm.logger.Infof("cloudvm.RefreshAddresses: using public addresses obtained from metadata, first one is: %v...", firstAddress(cvm.addrs))
	if cvm.includePrivateAddresses {
		priv, err := cvm.getPrivateAddr(ctx, time.Second)
		if err != nil {
			return err
		}
		cvm.addrs = append(cvm.addrs, priv...)
		cvm.logger.Infof("cloudvm.RefreshAddresses: also using private addresses obtained from metadata, first one is: %v...", firstAddress(priv))
	}
	return nil
}

func filterForProtocol(protocol string, addrs []net.Addr) []net.Addr {
	r := []net.Addr{}
	for _, a := range addrs {
		// If the address contains no protocol or its protocol is 'ip' then
		// it is assumed that it can support any protocol.
		if p := a.Network(); len(p) == 0 || p == "ip" || p == protocol {
			r = append(r, a)
		}
	}
	return r
}

// ChooseAddresses implements rpc.AddressChooser.
func (cvm *CloudVM) ChooseAddresses(protocol string, candidates []net.Addr) ([]net.Addr, error) {
	cvm.mu.Lock()
	defer cvm.mu.Unlock()
	forProtocol := filterForProtocol(protocol, cvm.addrs)
	if cvm.includePrivateAddresses {
		return append(forProtocol, filterForProtocol(protocol, candidates)...), nil
	}
	return forProtocol, nil
}
