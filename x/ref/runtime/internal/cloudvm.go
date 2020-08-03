// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"context"
	"fmt"
	"net"
	"runtime/debug"
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
	mu                            sync.Mutex
	dnsname                       net.Addr    // GUARDED_BY(mu)
	ipaddrs                       []net.Addr  // GUARDED_BY(mu)
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
// provides a Refresh method to update addresses based on current metadata
// settings. If PublicAddress is set it is used, then if a LiteralDNSName
// is set then it is used before finally trying to obtain public and private
// addresses from the virtualization/cloud provider environment's metadata.
func InitCloudVM(ctx context.Context, logger logging.Logger, fl *flags.VirtualizedFlags) (*CloudVM, error) {
	once.Do(func() {
		cvm, cvmErr = newCloudVM(ctx, logger, fl)
	})
	return cvm, cvmErr
}

func newCloudVM(ctx context.Context, logger logging.Logger, fl *flags.VirtualizedFlags) (*CloudVM, error) {
	cvm := &CloudVM{
		cfg:    fl,
		logger: logger,
	}
	if fl.VirtualizationProvider == flags.Native {
		return cvm, nil
	}
	if len(fl.LiteralDNSName) > 0 {
		cvm.dnsname = netstate.NewNetAddr(
			fl.PublicProtocol.Protocol,
			fl.LiteralDNSName)
	}
	if pa := fl.PublicAddress; len(pa.String()) > 0 {
		if len(pa.IP) > 0 {
			cvm.ipaddrs = make([]net.Addr, len(pa.IP))
			for i, a := range pa.IP {
				cvm.ipaddrs[i] = a
			}
		} else {
			addr := netstate.NewNetAddr(
				fl.PublicProtocol.Protocol,
				net.JoinHostPort(pa.Address, pa.Port))
			cvm.ipaddrs = []net.Addr{addr}
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

	var wg sync.WaitGroup
	var aws, gcp bool
	wg.Add(2)
	go func() {
		aws = cloudvm.OnAWS(ctx, time.Second)
		wg.Done()
	}()
	go func() {
		gcp = cloudvm.OnGCP(ctx, time.Second)
		wg.Done()
	}()
	wg.Wait()
	switch {
	case aws:
		isaws()
	case gcp:
		isgcp()
	default:
		return nil, fmt.Errorf("running on unsupported virtualization provider")
	}
	return refresh()
}

// RefreshAddresses updates the addresses from the viurtualization/cloud
// providers metadata if appropriate to do so.
func (cvm *CloudVM) RefreshAddresses(ctx context.Context) error {
	cvm.mu.Lock()
	defer cvm.mu.Unlock()
	if cvm.dnsname != nil || len(cvm.ipaddrs) > 0 {
		return nil
	}
	pub, err := cvm.getPublicAddr(ctx, time.Second)
	if err != nil {
		return err
	}
	priv, err := cvm.getPublicAddr(ctx, time.Second)
	if err != nil {
		return err
	}
	cvm.addrs = append(pub, priv...)
	return nil
}

// ChooseAddresses implements rpc.AddressChooser.
func (cvm *CloudVM) ChooseAddresses(protocol string, candidates []net.Addr) ([]net.Addr, error) {
	cvm.mu.Lock()
	defer cvm.mu.Unlock()
	debug.PrintStack()
	if len(cvm.ipaddrs) > 0 {
		cvm.logger.Infof("cloudvm.ChooseAddresses: returning public IP set on the command line")
		return cvm.ipaddrs, nil
	}
	if cvm.dnsname != nil {
		cvm.logger.Infof("cloudvm.ChooseAddresses: returning dns name set on the command line")
		return []net.Addr{cvm.dnsname}, nil
	}
	if len(cvm.addrs) > 0 {
		cvm.logger.Infof("cloudvm.ChooseAddresses: returning public and private addresses obtained from the cloud provider")
		return cvm.addrs, nil
	}
	cvm.logger.Infof("cloudvm.ChooseAddresses: returning original candidates")
	return candidates, nil
}
