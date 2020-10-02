// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package mdns implements mDNS plugin for discovery service.
//
// In order to support discovery of a specific vanadium service, an instance
// is advertised in two ways - one as a vanadium service and the other as a
// subtype of vanadium service.
//
// For example, a vanadium printer service is advertised as
//
//    v23._tcp.local.
//    _<printer_service_uuid>._sub._v23._tcp.local.
package mdns

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pborman/uuid"
	mdns "github.com/vanadium/go-mdns-sd"

	"v.io/v23/context"
	"v.io/v23/discovery"

	"v.io/x/lib/netconfig"
	idiscovery "v.io/x/ref/lib/discovery"
)

const (
	v23ServiceName    = "v23"
	serviceNameSuffix = "._sub._" + v23ServiceName

	// Use short attribute names due to the txt record size limit.
	attrInterface  = "_i"
	attrAddresses  = "_a"
	attrEncryption = "_e"
	attrHash       = "_h"
	attrTimestamp  = "_t"
	attrDirAddrs   = "_d"
	attrStatus     = "_s"

	// The prefix for attribute names for encoded attachments.
	attrAttachmentPrefix = "__"

	// The prefix for attribute names for encoded large txt records.
	attrLargeTxtPrefix = "_x"

	// RFC 6763 limits each DNS txt record to 255 bytes and recommends to not have
	// the cumulative size be larger than 1300 bytes. We limit the total size further
	// since the underlying mdns package allows records smaller than that.
	maxTxtRecordLen       = 255
	maxTotalTxtRecordsLen = 1184
)

var (
	errMaxTxtRecordLenExceeded = errors.New("max txt record size exceeded")
)

type plugin struct {
	ctx    *context.T
	closed chan struct{}
	wg     sync.WaitGroup

	mdns      *mdns.MDNS
	adStopper *idiscovery.Trigger

	subscriptionRefreshTime time.Duration
	subscriptionWaitTime    time.Duration
	subscriptionMu          sync.Mutex
	subscription            map[string]subscription // GUARDED_BY(subscriptionMu)
}

type subscription struct {
	count            int
	lastSubscription time.Time
}

func interfaceNameToServiceName(interfaceName string) string {
	serviceUUID := idiscovery.NewServiceUUID(interfaceName)
	return uuid.UUID(serviceUUID).String() + serviceNameSuffix
}

func (p *plugin) Advertise(ctx *context.T, adinfo *idiscovery.AdInfo, done func()) (err error) {
	serviceName := interfaceNameToServiceName(adinfo.Ad.InterfaceName)
	// We use the instance uuid as the host name so that we can get the instance uuid
	// from the lost service instance, which has no txt records at all.
	hostName := encodeAdID(&adinfo.Ad.Id)
	txt, err := newTxtRecords(adinfo)
	if err != nil {
		done()
		return err
	}

	// Announce the service.
	err = p.mdns.AddService(serviceName, hostName, 0, txt...)
	if err != nil {
		done()
		return err
	}

	// Announce it as v23 service as well so that we can discover
	// all v23 services through mDNS.
	err = p.mdns.AddService(v23ServiceName, hostName, 0, txt...)
	if err != nil {
		done()
		return err
	}
	stop := func() {
		p.mdns.RemoveService(serviceName, hostName, 0, txt...)    //nolint:errcheck
		p.mdns.RemoveService(v23ServiceName, hostName, 0, txt...) //nolint:errcheck
		done()
	}
	p.adStopper.Add(stop, ctx.Done())
	return nil
}

func (p *plugin) Scan(ctx *context.T, interfaceName string, callback func(*idiscovery.AdInfo), done func()) error {
	var serviceName string
	if len(interfaceName) == 0 {
		serviceName = v23ServiceName
	} else {
		serviceName = interfaceNameToServiceName(interfaceName)
	}

	go func() {
		defer done()

		p.subscribeToService(serviceName)
		watcher, stopWatcher := p.mdns.ServiceMemberWatch(serviceName)
		defer func() {
			stopWatcher()
			p.unsubscribeFromService(serviceName)
		}()

		for {
			var service mdns.ServiceInstance
			select {
			case service = <-watcher:
			case <-time.After(p.subscriptionRefreshTime):
				p.refreshSubscription(serviceName)
				continue
			case <-ctx.Done():
				return
			}
			adinfo, err := newAdInfo(service)
			if err != nil {
				ctx.Error(err)
				continue
			}
			callback(adinfo)
		}
	}()
	return nil
}

func (p *plugin) Close() {
	close(p.closed)
	p.wg.Wait()
}

func (p *plugin) watchNetConfig() {
	defer p.wg.Done()

	// Watch the network configuration so that we can make MDNS reattach to
	// interfaces when the network changes.
	for {
		ch, err := netconfig.NotifyChange()
		if err != nil {
			p.ctx.Error(err)
			return
		}
		select {
		case <-ch:
			if _, err := p.mdns.ScanInterfaces(); err != nil {
				p.ctx.Error(err)
			}
		case <-p.closed:
			return
		}
	}
}

func (p *plugin) subscribeToService(serviceName string) {
	p.subscriptionMu.Lock()
	sub := p.subscription[serviceName]
	sub.count++
	p.subscription[serviceName] = sub
	p.subscriptionMu.Unlock()
	p.refreshSubscription(serviceName)
}

func (p *plugin) unsubscribeFromService(serviceName string) {
	p.subscriptionMu.Lock()
	sub := p.subscription[serviceName]
	sub.count--
	if sub.count == 0 {
		delete(p.subscription, serviceName)
		p.mdns.UnsubscribeFromService(serviceName)
	} else {
		p.subscription[serviceName] = sub
	}
	p.subscriptionMu.Unlock()
}

func (p *plugin) refreshSubscription(serviceName string) {
	p.subscriptionMu.Lock()
	sub, ok := p.subscription[serviceName]
	if !ok {
		p.subscriptionMu.Unlock()
		return
	}
	// Subscribe to the service again if we haven't refreshed in a while.
	if time.Since(sub.lastSubscription) > p.subscriptionRefreshTime {
		p.mdns.SubscribeToService(serviceName)
		// Wait a bit to learn about neighborhood.
		time.Sleep(p.subscriptionWaitTime)
		sub.lastSubscription = time.Now()
	}
	p.subscription[serviceName] = sub
	p.subscriptionMu.Unlock()
}

type txtRecords struct {
	size int
	txts []string
}

type byTxtSize []txtRecords

func (r byTxtSize) Len() int           { return len(r) }
func (r byTxtSize) Swap(i, j int)      { r[i], r[j] = r[j], r[i] }
func (r byTxtSize) Less(i, j int) bool { return r[i].size < r[j].size }

func sizeOfTxtRecords(txts []string) int {
	n := 0
	for _, txt := range txts {
		n += len(txt)
	}
	return n
}

func appendTxtRecord(txts []string, k string, v interface{}, xseq int) ([]string, int) {
	var buf bytes.Buffer
	buf.WriteString(k)
	buf.WriteByte('=')
	if s, ok := v.(string); ok {
		buf.WriteString(s)
	} else {
		buf.Write(v.([]byte))
	}
	kvTxts, xseq := maybeSplitTxtRecord(buf.String(), xseq)
	return append(txts, kvTxts...), xseq
}

func packTxtRecords(status idiscovery.AdStatus, vtxts ...[]string) ([]string, error) {
	var packed []string
	if status != idiscovery.AdReady {
		packed, _ = appendTxtRecord(packed, attrStatus, strconv.Itoa(int(status)), 0)
	}
	for _, txts := range vtxts {
		packed = append(packed, txts...)
	}
	if sizeOfTxtRecords(packed) > maxTotalTxtRecordsLen {
		return nil, errMaxTxtRecordLenExceeded
	}
	return packed, nil
}

func newTxtRecords(adinfo *idiscovery.AdInfo) ([]string, error) {
	core, xseq := appendTxtRecord(nil, attrInterface, adinfo.Ad.InterfaceName, 0)
	core, xseq = appendTxtRecord(core, attrHash, adinfo.Hash[:], xseq)
	core, xseq = appendTxtRecord(core, attrTimestamp, idiscovery.EncodeTimestamp(adinfo.TimestampNs), xseq)
	coreLen := sizeOfTxtRecords(core)

	dir, xseq := appendTxtRecord(nil, attrDirAddrs, idiscovery.PackAddresses(adinfo.DirAddrs), xseq)
	dirLen := sizeOfTxtRecords(dir)

	required, xseq := appendTxtRecord(nil, attrAddresses, idiscovery.PackAddresses(adinfo.Ad.Addresses), xseq)
	for k, v := range adinfo.Ad.Attributes {
		required, xseq = appendTxtRecord(required, k, v, xseq)
	}
	if adinfo.EncryptionAlgorithm != idiscovery.NoEncryption {
		enc := idiscovery.PackEncryptionKeys(adinfo.EncryptionAlgorithm, adinfo.EncryptionKeys)
		required, xseq = appendTxtRecord(required, attrEncryption, enc, xseq)
	}
	requiredLen := sizeOfTxtRecords(required)

	remainingLen := maxTotalTxtRecordsLen - coreLen - requiredLen

	// Short-cut for a very large advertisement.
	if remainingLen < 0 {
		return packTxtRecords(idiscovery.AdNotReady, core, dir)
	}

	extra := make([]txtRecords, 0, len(adinfo.Ad.Attachments))
	extraLen := 0
	for k, v := range adinfo.Ad.Attachments {
		if xseq >= maxNumLargeTxtRecords {
			break
		}
		if len(v) > remainingLen {
			continue
		}
		var txts []string
		txts, xseq = appendTxtRecord(nil, attrAttachmentPrefix+k, v, xseq)
		n := sizeOfTxtRecords(txts)
		extra = append(extra, txtRecords{n, txts})
		extraLen += n
	}

	if len(extra) == len(adinfo.Ad.Attachments) && extraLen <= remainingLen {
		// The advertisement can fit in a packet.
		full := make([]string, 0, len(extra))
		for _, r := range extra {
			full = append(full, r.txts...)
		}
		return packTxtRecords(idiscovery.AdReady, core, required, full)
	}

	const statusTxtRecordLen = 4 // _s=<s>, where <s> is one byte.
	remainingLen -= dirLen + statusTxtRecordLen
	if remainingLen < 0 {
		// Not all required fields can fit in a packet.
		return packTxtRecords(idiscovery.AdNotReady, core, dir)
	}

	sort.Sort(byTxtSize(extra))

	selected := make([]string, 0, len(extra))
	for _, r := range extra {
		if remainingLen >= r.size {
			selected = append(selected, r.txts...)
			remainingLen -= r.size
		}
		if remainingLen <= 0 {
			break
		}
	}
	return packTxtRecords(idiscovery.AdPartiallyReady, core, dir, required, selected)
}

func newAdInfo(service mdns.ServiceInstance) (*idiscovery.AdInfo, error) { //nolint:gocyclo
	// Note that service.Name starts with a host name, which is the instance uuid.
	p := strings.SplitN(service.Name, ".", 2)
	if len(p) < 1 {
		return nil, fmt.Errorf("invalid service name: %s", service.Name)
	}

	adinfo := &idiscovery.AdInfo{}
	if err := decodeAdID(p[0], &adinfo.Ad.Id); err != nil {
		return nil, fmt.Errorf("invalid host name: %v", err)
	}

	if len(service.SrvRRs) == 0 && len(service.TxtRRs) == 0 {
		adinfo.Lost = true
		return adinfo, nil
	}

	adinfo.Ad.Attributes = make(discovery.Attributes)
	adinfo.Ad.Attachments = make(discovery.Attachments)
	for _, rr := range service.TxtRRs {
		txts, err := maybeJoinTxtRecords(rr.Txt)
		if err != nil {
			return nil, err
		}

		for _, txt := range txts {
			kv := strings.SplitN(txt, "=", 2)
			if len(kv) != 2 {
				return nil, fmt.Errorf("invalid txt record: %s", txt)
			}
			switch k, v := kv[0], kv[1]; k {
			case attrInterface:
				adinfo.Ad.InterfaceName = v
			case attrAddresses:
				if adinfo.Ad.Addresses, err = idiscovery.UnpackAddresses([]byte(v)); err != nil {
					return nil, err
				}
			case attrEncryption:
				if adinfo.EncryptionAlgorithm, adinfo.EncryptionKeys, err = idiscovery.UnpackEncryptionKeys([]byte(v)); err != nil {
					return nil, err
				}
			case attrHash:
				copy(adinfo.Hash[:], v)
			case attrTimestamp:
				if adinfo.TimestampNs, err = idiscovery.DecodeTimestamp([]byte(v)); err != nil {
					return nil, err
				}
			case attrDirAddrs:
				if adinfo.DirAddrs, err = idiscovery.UnpackAddresses([]byte(v)); err != nil {
					return nil, err
				}
			case attrStatus:
				status, err := strconv.Atoi(v)
				if err != nil {
					return nil, err
				}
				adinfo.Status = idiscovery.AdStatus(status)
			default:
				if strings.HasPrefix(k, attrAttachmentPrefix) {
					adinfo.Ad.Attachments[k[len(attrAttachmentPrefix):]] = []byte(v)
				} else {
					adinfo.Ad.Attributes[k] = v
				}
			}
		}
	}
	return adinfo, nil
}

func New(ctx *context.T, host string) (idiscovery.Plugin, error) {
	return newWithLoopback(ctx, host, 0, false)
}

func newWithLoopback(ctx *context.T, host string, port int, loopback bool) (idiscovery.Plugin, error) {
	switch {
	case len(host) == 0:
		// go-mdns-sd doesn't answer when the host name is not set.
		// Assign a default one if not given.
		host = "v23()"
	case host == "localhost":
		// localhost is a default host name in many places.
		// Add a suffix for uniqueness.
		host += "()"
	}
	var v4addr, v6addr string
	if port > 0 {
		v4addr = fmt.Sprintf("224.0.0.251:%d", port)
		v6addr = fmt.Sprintf("[FF02::FB]:%d", port)
	}
	m, err := mdns.NewMDNS(host, v4addr, v6addr, loopback, 0)
	if err != nil {
		// The name may not have been unique. Try one more time with a unique
		// name. NewMDNS will replace the "()" with "(hardware mac address)".
		if len(host) > 0 && !strings.HasSuffix(host, "()") {
			m, err = mdns.NewMDNS(host+"()", "", "", loopback, 0)
		}
		if err != nil {
			return nil, err
		}
	}
	p := plugin{
		ctx:       ctx,
		closed:    make(chan struct{}),
		mdns:      m,
		adStopper: idiscovery.NewTrigger(),
		// TODO(jhahn): Figure out a good subscription refresh time.
		subscriptionRefreshTime: 5 * time.Second,
		subscription:            make(map[string]subscription),
	}
	if loopback {
		p.subscriptionWaitTime = 5 * time.Millisecond
	} else {
		p.subscriptionWaitTime = 50 * time.Millisecond
	}
	p.wg.Add(1)
	go p.watchNetConfig()
	return &p, nil
}
