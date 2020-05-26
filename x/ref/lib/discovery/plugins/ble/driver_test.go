// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ble

import (
	"errors"
	"fmt"
	"sync"

	"v.io/v23/context"
)

// mockDriver is a driver for testing BLE plugin without a real driver.
// Drivers within a "neighborhood" can advertise and discover each other.
type mockDriver struct {
	mu       sync.Mutex
	services map[string]map[string][]byte // GUARDED_BY(mu)

	scanUuids   map[string]struct{} // GUARDED_BY(mu)
	scanHandler ScanHandler         // GUARDED_BY(mu)

	broadcasting chan<- *mockPacket
	scanning     <-chan *mockPacket
	done         <-chan struct{}
}

type mockPacket struct {
	driver          *mockDriver
	uuid            string
	characteristics map[string][]byte
}

func (d *mockDriver) AddService(uuid string, characteristics map[string][]byte) error {
	d.mu.Lock()
	if _, ok := d.services[uuid]; ok {
		return fmt.Errorf("already being advertised: %s", uuid)
	}
	d.services[uuid] = characteristics
	d.mu.Unlock()
	d.broadcasting <- &mockPacket{d, uuid, characteristics}
	return nil
}

func (d *mockDriver) RemoveService(uuid string) {
	d.mu.Lock()
	delete(d.services, uuid)
	d.mu.Unlock()
}

func (d *mockDriver) StartScan(uuids []string, baseUUID, maskUUID string, handler ScanHandler) error {
	d.mu.Lock()
	if d.scanUuids != nil {
		return errors.New("scan already started")
	}
	d.scanUuids = make(map[string]struct{})
	for _, uuid := range uuids {
		d.scanUuids[uuid] = struct{}{}
	}
	d.scanHandler = handler
	d.mu.Unlock()
	d.broadcasting <- &mockPacket{driver: d}
	return nil
}

func (d *mockDriver) StopScan() {
	d.mu.Lock()
	d.scanUuids = nil
	d.scanHandler = nil
	d.mu.Unlock()
}

func (d *mockDriver) DebugString() string { return "mock" }

func (d *mockDriver) scanLoop() {
	for {
		select {
		case packet := <-d.scanning:
			if len(packet.uuid) == 0 {
				d.rebroacast()
			} else {
				d.discover(packet.uuid, packet.characteristics)
			}
		case <-d.done:
			return
		}
	}
}

func (d *mockDriver) rebroacast() {
	d.mu.Lock()
	for uuid, cs := range d.services {
		d.broadcasting <- &mockPacket{d, uuid, cs}
	}
	d.mu.Unlock()
}

func (d *mockDriver) discover(uuid string, characteristics map[string][]byte) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.scanHandler == nil {
		return
	}
	if _, ok := d.scanUuids[uuid]; ok || len(d.scanUuids) == 0 {
		d.scanHandler.OnDiscovered(uuid, characteristics, 0)
	}
}

type mockNeighborhood struct {
	mu      sync.Mutex
	drivers map[*mockDriver]chan *mockPacket // GUARDED_BY(mu)

	broadcasting chan *mockPacket
	done         chan struct{}
}

func (n *mockNeighborhood) newDriver(ctx *context.T, host string) (Driver, error) {
	scanning := make(chan *mockPacket, 100)
	driver := &mockDriver{
		services:     make(map[string]map[string][]byte),
		broadcasting: n.broadcasting,
		scanning:     scanning,
		done:         n.done,
	}
	go driver.scanLoop()

	n.mu.Lock()
	n.drivers[driver] = scanning
	n.mu.Unlock()
	return driver, nil
}

func (n *mockNeighborhood) broadcastLoop() {
	for {
		select {
		case packet := <-n.broadcasting:
			n.mu.Lock()
			for d, ch := range n.drivers {
				if d == packet.driver {
					continue
				}
				ch <- packet
			}
			n.mu.Unlock()
		case <-n.done:
			return
		}
	}
}

func (n *mockNeighborhood) shutdown() {
	close(n.done)
}

func newNeighborhood() *mockNeighborhood {
	n := &mockNeighborhood{
		drivers:      make(map[*mockDriver]chan *mockPacket),
		broadcasting: make(chan *mockPacket, 1000),
		done:         make(chan struct{}),
	}
	go n.broadcastLoop()

	SetDriverFactory(n.newDriver)
	return n
}
