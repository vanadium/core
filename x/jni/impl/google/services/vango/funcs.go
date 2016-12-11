// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vango

import (
	"bytes"
	"fmt"
	"io"
	"time"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/discovery"
	"v.io/v23/naming"
	"v.io/v23/options"
	"v.io/v23/rpc"
	"v.io/v23/security"
	libdiscovery "v.io/x/ref/lib/discovery"
	"v.io/x/ref/lib/stats"
)

const (
	rpcTimeout    = 10 * time.Second
	tcpServerName = "tmp/vango/tcp"
	btServerName  = "tmp/vango/bt"
	interfaceName = "v.io/x/jni/impl/google/services/vango.EchoServer"
	vangoStat     = "vango"
)

var (
	bleServerName = naming.Endpoint{Protocol: "ble"}.Name()

	// vangoFuncs is a map containing go functions keys by unique strings
	// intended to be run by java/android applications using Vango.run(key).
	// Users must add function entries to this map and rebuild lib/android-lib in
	// the vanadium java repository.
	vangoFuncs = map[string]func(*context.T, io.Writer) error{
		"tcp-client":  tcpClientFunc,
		"tcp-server":  tcpServerFunc,
		"bt-client":   btClientFunc,
		"bt-server":   btServerFunc,
		"ble-client":  bleClientFunc,
		"ble-server":  bleServerFunc,
		"all":         AllFunc,
		"btdiscovery": btAndDiscoveryFunc,
	}
)

func tcpServerFunc(ctx *context.T, _ io.Writer) error {
	ctx = v23.WithListenSpec(ctx, rpc.ListenSpec{Proxy: "proxy"})
	return runServer(ctx, tcpServerName)
}

func tcpClientFunc(ctx *context.T, _ io.Writer) error {
	return runClient(ctx, tcpServerName)
}

func btServerFunc(ctx *context.T, _ io.Writer) error {
	ctx = v23.WithListenSpec(ctx, rpc.ListenSpec{Addrs: rpc.ListenAddrs{{Protocol: "bt", Address: "/0"}}})
	return runServer(ctx, btServerName)
}

func btClientFunc(ctx *context.T, _ io.Writer) error {
	return runClient(ctx, btServerName)
}

func bleServerFunc(ctx *context.T, _ io.Writer) error {
	ctx = v23.WithListenSpec(ctx, rpc.ListenSpec{Addrs: rpc.ListenAddrs{{Protocol: "ble", Address: "na"}}})
	return runServer(ctx, "")
}

func bleClientFunc(ctx *context.T, _ io.Writer) error {
	return runClient(ctx, bleServerName)
}

func btAndDiscoveryFunc(ctx *context.T, w io.Writer) error {
	bothf := func(ctx *context.T, w io.Writer, format string, args ...interface{}) {
		fmt.Fprintf(w, format, args...)
		ctx.Infof(format, args...)
	}
	defer bothf(ctx, w, "finishing!")

	dis, err := v23.NewDiscovery(ctx)
	if err != nil {
		bothf(ctx, w, "Can't create discovery %v", err)
		return err
	}

	ctx = v23.WithListenSpec(ctx, rpc.ListenSpec{Addrs: rpc.ListenAddrs{{Protocol: "bt", Address: "/0"}}})
	_, server, err := v23.WithNewServer(ctx, "", &echoServer{}, security.AllowEveryone())
	if err != nil {
		bothf(ctx, w, "Can't create server %v", err)
		return err
	}
	ctx.Infof("Server listening on %v", server.Status().Endpoints)
	ctx.Infof("Server listen errors: %v", server.Status().ListenErrors)

	interfaces := []string{
		"v.io/x/jni/impl/google/services/vango/Echo",
		"v.io/x/jni/impl/google/services/vango/Echo2",
		"v.io/x/jni/impl/google/services/vango/Echo3",
		"v.io/x/jni/impl/google/services/vango/Echo4",
	}
	type adstate struct {
		ad   *discovery.Advertisement
		stop func()
	}
	ads := []adstate{}
	for _, name := range interfaces {
		ad := &discovery.Advertisement{
			InterfaceName: name,
			Attributes: discovery.Attributes{
				"one":   "A value of some kind",
				"two":   "Yet another value",
				"three": "More and more",
				"four":  "This is insane",
			},
		}
		nctx, ncancel := context.WithCancel(ctx)
		ch, err := libdiscovery.AdvertiseServer(nctx, dis, server, "", ad, nil)
		if err != nil {
			bothf(nctx, w, "Can't advertise server %v", err)
			return err
		}

		stop := func() {
			ncancel()
			<-ch
		}
		ads = append(ads, adstate{ad, stop})
	}

	type updateState struct {
		ch   <-chan discovery.Update
		stop func()
	}
	var updates []updateState
	for _, name := range interfaces {
		nctx, ncancel := context.WithCancel(ctx)
		u, err := dis.Scan(nctx, `v.InterfaceName="`+name+`"`)
		if err != nil {
			bothf(nctx, w, "Can't scan %v", err)
			return err
		}
		stop := func() {
			ncancel()
		}
		updates = append(updates, updateState{u, stop})
	}

	for _, u := range updates[1:] {
		go func(up updateState) {
			for _ = range up.ch {
			}
		}(u)
	}

	makeopt := func(ad discovery.Advertisement) options.Preresolved {
		me := &naming.MountEntry{
			IsLeaf: true,
		}
		for _, a := range ad.Addresses {
			addr, _ := naming.SplitAddressName(a)
			me.Servers = append(me.Servers, naming.MountedServer{
				Server: addr,
			})
		}
		return options.Preresolved{Resolution: me}
	}

	alive := map[discovery.AdId]options.Preresolved{}
	ticker := time.NewTicker(time.Second)
	for {
		select {
		case <-ticker.C:
			if len(alive) == 0 {
				bothf(ctx, w, "No live connections to dial.")
			}
			for _, opt := range alive {
				dialtime := options.ConnectionTimeout(5 * time.Second)
				channeltime := options.ChannelTimeout(2 * time.Second)
				data := make([]byte, 1024)
				summary, err := runTimedCall(ctx, "A timed call.", string(data), opt, dialtime, channeltime)
				if err != nil {
					bothf(ctx, w, "failed call %s, %v, %v", summary, err, opt.Resolution.Servers)
				} else {
					bothf(ctx, w, "succeeded call: %s, %v", summary, opt.Resolution.Servers)
				}
			}

		case u := <-updates[0].ch:
			if u.IsLost() {
				bothf(ctx, w, "lost %v", u.Addresses())
				delete(alive, u.Id())
			} else {
				bothf(ctx, w, "found %v", u.Addresses())
				alive[u.Id()] = makeopt(u.Advertisement())
			}
		}
	}
}

// AllFunc runs a server, advertises it, scans for other servers and makes an
// Echo RPC to every advertised remote server.
func AllFunc(ctx *context.T, output io.Writer) error {
	ls := rpc.ListenSpec{Proxy: "proxy"}
	addRegisteredProto(&ls, "tcp", ":0")
	addRegisteredProto(&ls, "bt", "/0")
	fmt.Fprintf(output, "Listening on: %+v (and proxy)\n", ls.Addrs)
	ctx, server, err := v23.WithNewServer(
		v23.WithListenSpec(ctx, ls),
		mountName(ctx, "all"),
		&echoServer{},
		security.AllowEveryone())
	if err != nil {
		return err
	}
	ad := &discovery.Advertisement{
		InterfaceName: interfaceName,
		Attributes: discovery.Attributes{
			"Hello": "There",
		},
	}
	d, err := v23.NewDiscovery(ctx)
	if err != nil {
		return err
	}
	stoppedAd, err := libdiscovery.AdvertiseServer(ctx, d, server, "", ad, nil)
	if err != nil {
		return err
	}
	updates, err := d.Scan(ctx, "v.InterfaceName=\""+interfaceName+"\"")
	if err != nil {
		return err
	}
	var (
		status      = server.Status()
		counter     = 0
		peerByAdId  = make(map[discovery.AdId]*peer)
		lastCall    = make(map[discovery.AdId]time.Time)
		callResults = make(chan string)
		activeCalls = 0
		quit        = false
		myaddrs     = serverAddrs(status)
		ticker      = time.NewTicker(time.Second)
		call        = func(p *peer) {
			counter++
			activeCalls++
			lastCall[p.adId] = time.Now()
			go func(msg string) {
				summary, err := p.call(ctx, msg)
				if err != nil {
					ctx.Infof("Failed to call [%v]: %v", p.description, err)
					callResults <- ""
					return
				}
				callResults <- summary
			}(fmt.Sprintf("Hello #%d", counter))
		}
		statRequest = make(chan chan<- string)
	)
	defer ticker.Stop()
	stats.NewStringFunc(vangoStat, func() string {
		r := make(chan string)
		statRequest <- r
		return <-r
	})
	defer stats.Delete(vangoStat)
	fmt.Fprintln(output, "My AdID:", ad.Id)
	fmt.Fprintln(output, "My addrs:", myaddrs)
	ctx.Infof("SERVER STATUS: %+v", status)
	for !quit {
		select {
		case <-ctx.Done():
			quit = true
		case <-status.Dirty:
			status = server.Status()
			newaddrs := serverAddrs(status)
			changed := len(newaddrs) != len(myaddrs)
			if !changed {
				for i := range newaddrs {
					if newaddrs[i] != myaddrs[i] {
						changed = true
						break
					}
				}
			}
			if changed {
				myaddrs = newaddrs
				fmt.Fprintln(output, "My addrs:", myaddrs)
			}
			ctx.Infof("SERVER STATUS: %+v", status)
		case u, scanning := <-updates:
			if !scanning {
				fmt.Fprintln(output, "SCANNING STOPPED")
				quit = true
				break
			}
			if u.IsLost() {
				if p, ok := peerByAdId[u.Id()]; ok {
					fmt.Fprintln(output, "LOST:", p.description)
				}
				delete(peerByAdId, u.Id())
				delete(lastCall, u.Id())
				break
			}
			p, err := newPeer(ctx, u)
			if err != nil {
				ctx.Info(err)
				break
			}
			peerByAdId[p.adId] = p
			fmt.Fprintln(output, "FOUND:", p.description)
			call(p)
		case r := <-callResults:
			activeCalls--
			if len(r) > 0 {
				fmt.Fprintln(output, r)
			}
		case <-stoppedAd:
			fmt.Fprintln(output, "STOPPED ADVERTISING")
			stoppedAd = nil
		case <-ticker.C:
			// Call all peers that haven't been called in a while
			now := time.Now()
			for id, t := range lastCall {
				if now.Sub(t) > rpcTimeout {
					call(peerByAdId[id])
				}
			}
		case s := <-statRequest:
			idx := 1
			ret := new(bytes.Buffer)
			fmt.Fprintln(ret, "ACTIVE CALLS:", activeCalls)
			fmt.Fprintln(ret, "PEERS")
			for id, p := range peerByAdId {
				fmt.Fprintf(ret, "%2d) %s -- %v\n", idx, p.description, lastCall[id])
			}
			s <- ret.String()
		}
	}
	fmt.Println(output, "EXITING: Cleaning up")
	for activeCalls > 0 {
		<-callResults
		activeCalls--
	}
	// Exhaust the scanned updates queue.
	// (The channel will be closed as a by-product of the context being Done).
	for range updates {
	}
	fmt.Fprintln(output, "EXITING: Done")
	return nil
}
