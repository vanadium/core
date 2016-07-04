// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testutil

import (
	"fmt"
	"reflect"
	"sync"
	"time"

	"v.io/v23/context"
	"v.io/v23/discovery"

	idiscovery "v.io/x/ref/lib/discovery"
)

func Advertise(ctx *context.T, p idiscovery.Plugin, adinfos ...*idiscovery.AdInfo) (func(), error) {
	ctx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	for _, adinfo := range adinfos {
		wg.Add(1)
		if err := p.Advertise(ctx, adinfo, wg.Done); err != nil {
			cancel()
			return nil, fmt.Errorf("Advertise failed: %v", err)
		}
	}
	stop := func() {
		cancel()
		wg.Wait()
	}
	return stop, nil
}

func Scan(ctx *context.T, p idiscovery.Plugin, interfaceName string) (<-chan *idiscovery.AdInfo, func(), error) {
	ctx, cancel := context.WithCancel(ctx)
	scanCh := make(chan *idiscovery.AdInfo)
	callback := func(ad *idiscovery.AdInfo) {
		select {
		case scanCh <- ad:
		case <-ctx.Done():
		}
	}
	var wg sync.WaitGroup
	wg.Add(1)
	if err := p.Scan(ctx, interfaceName, callback, wg.Done); err != nil {
		return nil, nil, fmt.Errorf("Scan failed: %v", err)
	}
	stop := func() {
		cancel()
		wg.Wait()
	}
	return scanCh, stop, nil
}

func ScanAndMatch(ctx *context.T, p idiscovery.Plugin, interfaceName string, wants ...idiscovery.AdInfo) error {
	const timeout = 10 * time.Second

	var adinfos []idiscovery.AdInfo
	for now := time.Now(); time.Since(now) < timeout; {
		var err error
		adinfos, err = doScan(ctx, p, interfaceName, len(wants))
		if err != nil {
			return err
		}
		if MatchFound(adinfos, wants...) {
			return nil
		}
	}
	return fmt.Errorf("Match failed; got %#v, but wanted %#v", adinfos, wants)
}

func doScan(ctx *context.T, p idiscovery.Plugin, interfaceName string, expectedAdInfos int) ([]idiscovery.AdInfo, error) {
	scanCh, stop, err := Scan(ctx, p, interfaceName)
	if err != nil {
		return nil, err
	}
	defer stop()

	adinfos := make([]idiscovery.AdInfo, 0, expectedAdInfos)
	for {
		var timer <-chan time.Time
		if len(adinfos) >= expectedAdInfos {
			timer = time.After(5 * time.Millisecond)
		}

		select {
		case adinfo := <-scanCh:
			adinfos = append(adinfos, *adinfo)
		case <-timer:
			return adinfos, nil
		}
	}
}

func MatchFound(adinfos []idiscovery.AdInfo, wants ...idiscovery.AdInfo) bool {
	return match(adinfos, false, wants...)
}

func MatchLost(adinfos []idiscovery.AdInfo, wants ...idiscovery.AdInfo) bool {
	return match(adinfos, true, wants...)
}

func match(adinfos []idiscovery.AdInfo, lost bool, wants ...idiscovery.AdInfo) bool {
	adinfoMap := make(map[discovery.AdId]idiscovery.AdInfo)
	for _, adinfo := range adinfos {
		adinfoMap[adinfo.Ad.Id] = adinfo
	}

	for _, want := range wants {
		adinfo, ok := adinfoMap[want.Ad.Id]
		if !ok {
			return false
		}
		if lost {
			if !adinfo.Lost {
				return false
			}
		} else {
			if !reflect.DeepEqual(adinfo, want) {
				return false
			}
		}
		delete(adinfoMap, want.Ad.Id)
	}
	return len(adinfoMap) == 0
}

func WaitUntilMatchFound(ch <-chan *idiscovery.AdInfo, want idiscovery.AdInfo) error {
	return waitUntilMatches(ch, false, want)
}

func WaitUntilMatchLost(ch <-chan *idiscovery.AdInfo, want idiscovery.AdInfo) error {
	return waitUntilMatches(ch, true, want)
}

func waitUntilMatches(ch <-chan *idiscovery.AdInfo, lost bool, want idiscovery.AdInfo) error {
	timeout := time.After(10 * time.Second)

	for {
		select {
		case adinfo := <-ch:
			if match([]idiscovery.AdInfo{*adinfo}, lost, want) {
				return nil
			}
		case <-timeout:
			want.Lost = lost
			return fmt.Errorf("Match failed; got none, but wanted %#v", want)
		}
	}
}
