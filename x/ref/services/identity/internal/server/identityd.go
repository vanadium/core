// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// HTTP server that uses OAuth to create security.Blessings objects.
package server

import (
	"fmt"
	mrand "math/rand"
	"net"
	"net/http"
	"strconv"
	"strings"
	"syscall"
	"time"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/verror"
	"v.io/x/ref/lib/security/audit"
	"v.io/x/ref/lib/signals"
	"v.io/x/ref/services/discharger"
	"v.io/x/ref/services/identity/internal/auditor"
	"v.io/x/ref/services/identity/internal/blesser"
	"v.io/x/ref/services/identity/internal/caveats"
	"v.io/x/ref/services/identity/internal/dischargerlib"
	"v.io/x/ref/services/identity/internal/handlers"
	"v.io/x/ref/services/identity/internal/oauth"
	"v.io/x/ref/services/identity/internal/revocation"
	"v.io/x/ref/services/identity/internal/templates"
)

const (
	macaroonService   = "macaroon"
	dischargerService = "discharger"
)

type IdentityServer struct {
	oauthProvider      oauth.OAuthProvider
	auditor            audit.Auditor
	blessingLogReader  auditor.BlessingLogReader
	revocationManager  revocation.RevocationManager
	caveatSelector     caveats.CaveatSelector
	rootedObjectAddrs  []naming.Endpoint
	assetsPrefix       string
	mountNamePrefix    string
	dischargerLocation string
	registeredApps     handlers.RegisteredAppMap
}

// NewIdentityServer returns a IdentityServer that:
// - uses oauthProvider to authenticate users
// - auditor and blessingLogReader to audit the root principal and read audit logs
// - revocationManager to store revocation data and grant discharges
func NewIdentityServer(oauthProvider oauth.OAuthProvider, auditor audit.Auditor, blessingLogReader auditor.BlessingLogReader, revocationManager revocation.RevocationManager, caveatSelector caveats.CaveatSelector, assetsPrefix, mountNamePrefix, dischargerLocation string, registeredApps handlers.RegisteredAppMap) *IdentityServer {
	return &IdentityServer{
		oauthProvider:      oauthProvider,
		auditor:            auditor,
		blessingLogReader:  blessingLogReader,
		revocationManager:  revocationManager,
		caveatSelector:     caveatSelector,
		assetsPrefix:       assetsPrefix,
		mountNamePrefix:    mountNamePrefix,
		dischargerLocation: dischargerLocation,
		registeredApps:     registeredApps,
	}
}

// findUnusedPort finds an unused port and returns it. Of course, no guarantees
// are made that the port will actually be available by the time the caller
// gets around to binding to it. If no port can be found, (0, nil) is returned.
// If an error occurs while creating a socket, that error is returned and the
// other return value is 0.
func findUnusedPort() (int, error) {
	random := mrand.New(mrand.NewSource(time.Now().UnixNano()))
	for i := 0; i < 1000; i++ {
		fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, syscall.IPPROTO_TCP)
		if err != nil {
			return 0, err
		}

		port := int(1024 + random.Int31n(64512))
		sa := &syscall.SockaddrInet4{Port: port}
		err = syscall.Bind(fd, sa)
		syscall.Close(fd)
		if err == nil {
			return port, nil
		}
	}

	return 0, nil
}

func (s *IdentityServer) Serve(ctx, oauthCtx *context.T, externalHTTPAddr, httpAddr, tlsConfig string) {
	ctx, err := v23.WithPrincipal(ctx, audit.NewPrincipal(ctx, s.auditor))
	if err != nil {
		ctx.Panic(err)
	}
	oauthCtx, err = v23.WithPrincipal(oauthCtx, audit.NewPrincipal(oauthCtx, s.auditor))
	if err != nil {
		ctx.Panic(err)
	}
	httphost, httpport, err := net.SplitHostPort(httpAddr)
	if err != nil || httpport == "0" {
		httpportNum, err := findUnusedPort()
		if err != nil {
			ctx.Panic(err)
		}
		httpAddr = net.JoinHostPort(httphost, strconv.Itoa(httpportNum))
	}
	ctx, cancel := context.WithCancel(ctx)
	rpcServer, _, externalAddr := s.Listen(ctx, oauthCtx, externalHTTPAddr, httpAddr, tlsConfig)
	fmt.Printf("HTTP_ADDR=%s\n", externalAddr)
	if len(s.rootedObjectAddrs) > 0 {
		fmt.Printf("NAME=%s\n", s.rootedObjectAddrs[0].Name())
	}
	<-signals.ShutdownOnSignals(ctx)
	ctx.Infof("Received shutdown request.")
	cancel()
	<-rpcServer.Closed()
	ctx.Infof("Successfully stopped the rpc server.")
}

func (s *IdentityServer) Listen(ctx, oauthCtx *context.T, externalHTTPAddr, httpAddr, tlsConfig string) (rpc.Server, []string, string) {
	// json-encoded public key and blessing names of this server
	principal := v23.GetPrincipal(ctx)
	http.Handle("/auth/blessing-root", handlers.BlessingRoot{P: principal})

	rpcServer, published, err := s.setupBlessingServices(ctx, oauthCtx)
	if err != nil {
		ctx.Fatalf("Failed to setup vanadium services for blessing: %v", err)
	}

	externalHTTPAddr = httpAddress(externalHTTPAddr, httpAddr)

	http.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	n := "/auth/google/"
	args := oauth.HandlerArgs{
		Principal:          principal,
		Addr:               fmt.Sprintf("%s%s", externalHTTPAddr, n),
		BlessingLogReader:  s.blessingLogReader,
		RevocationManager:  s.revocationManager,
		DischargerLocation: s.dischargerLocation,
		MacaroonBlessingService: func() []string {
			status := rpcServer.Status()
			names := make([]string, len(status.Endpoints))
			for i, e := range status.Endpoints {
				names[i] = naming.JoinAddressName(e.Name(), macaroonService)
			}
			return names
		},
		OAuthProvider:  s.oauthProvider,
		CaveatSelector: s.caveatSelector,
		AssetsPrefix:   s.assetsPrefix,
	}
	if s.revocationManager != nil {
		args.DischargeServers = appendSuffixTo(published, dischargerService)
	}
	http.Handle(n, oauth.NewHandler(ctx, args))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		self, _ := principal.BlessingStore().Default()
		tmplArgs := struct {
			Self               security.Blessings
			DischargeServers   []string
			ListBlessingsRoute string
			AssetsPrefix       string
			Email              string
		}{
			Self:               self,
			DischargeServers:   args.DischargeServers,
			ListBlessingsRoute: oauth.ListBlessingsRoute,
			AssetsPrefix:       s.assetsPrefix,
		}
		if err := templates.Home.Execute(w, tmplArgs); err != nil {
			ctx.Info("Failed to render template:", err)
		}
	})
	ctx.Infof("Running HTTP server at: %v", externalHTTPAddr)
	go runHTTPSServer(ctx, httpAddr, tlsConfig)
	return rpcServer, published, externalHTTPAddr
}

func appendSuffixTo(objectname []string, suffix string) []string {
	names := make([]string, len(objectname))
	for i, o := range objectname {
		names[i] = naming.JoinAddressName(o, suffix)
	}
	return names
}

// Starts the Vanadium and HTTP services for blessing, and the Vanadium service for discharging.
// All Vanadium services are started on the same port.
func (s *IdentityServer) setupBlessingServices(ctx, oauthCtx *context.T) (rpc.Server, []string, error) {
	disp := newDispatcher()
	p := v23.GetPrincipal(ctx)
	b, _ := p.BlessingStore().Default()
	blessingNames := security.BlessingNames(p, b)
	if len(blessingNames) == 0 {
		return nil, nil, verror.ErrInternal.Errorf(ctx, "identity server has no blessings?")
	}
	if len(blessingNames) > 1 {
		return nil, nil, verror.ErrInternal.Errorf(ctx, "cannot configure identity server with >1 (%d = %v) blessings - not quite sure what names to select for the discharger service etc.", len(blessingNames), blessingNames)
	}
	objectAddr := naming.Join(s.mountNamePrefix, naming.EncodeAsNameElement(blessingNames[0]))
	ctx, server, err := v23.WithNewDispatchingServer(ctx, objectAddr, disp)
	if err != nil {
		return nil, nil, err
	}
	s.rootedObjectAddrs = server.Status().Endpoints
	var rootedObjectAddr string
	if naming.Rooted(objectAddr) {
		rootedObjectAddr = objectAddr
	} else if nsroots := v23.GetNamespace(ctx).Roots(); len(nsroots) >= 1 {
		rootedObjectAddr = naming.Join(nsroots[0], objectAddr)
	} else {
		rootedObjectAddr = s.rootedObjectAddrs[0].Name()
	}
	if s.dischargerLocation == "" {
		s.dischargerLocation = naming.Join(rootedObjectAddr, dischargerService)
	}
	ctx.Infof("Vanadium Blessing and discharger services will be published at %v", rootedObjectAddr)
	// Start the HTTP Handler for the OAuth2 access token based blesser.
	handlerParams := handlers.OAuthBlesserParams{
		OAuthProvider:      s.oauthProvider,
		BlessingDuration:   365 * 24 * time.Hour,
		RevocationManager:  s.revocationManager,
		DischargerLocation: s.dischargerLocation,
	}
	http.Handle("/auth/google/bless", handlers.NewOAuthBlessingHandler(oauthCtx, handlerParams, s.registeredApps))
	return server, []string{rootedObjectAddr}, nil
}

// newDispatcher returns a dispatcher for both the blessing and the discharging
// service.
func newDispatcher() *dispatcher {
	d := &dispatcher{
		m: map[string]interface{}{
			macaroonService:   blesser.NewMacaroonBlesserServer(),
			dischargerService: discharger.DischargerServer(dischargerlib.NewDischarger()),
		},
	}
	// Set up the glob invoker.
	var children []string
	for k := range d.m {
		children = append(children, k)
	}
	d.m[""] = rpc.ChildrenGlobberInvoker(children...)
	return d
}

type dispatcher struct {
	m map[string]interface{}
}

func (d *dispatcher) Lookup(ctx *context.T, suffix string) (interface{}, security.Authorizer, error) {
	if invoker := d.m[suffix]; invoker != nil {
		return invoker, security.AllowEveryone(), nil
	}
	return nil, nil, verror.ErrNoExist.Errorf(ctx, "does not exist: %s", suffix)
}

func runHTTPSServer(ctx *context.T, addr, tlsConfig string) {
	if len(tlsConfig) == 0 {
		ctx.Fatal("Please set the --tls-config flag")
	}
	paths := strings.Split(tlsConfig, ",")
	if len(paths) != 2 {
		ctx.Fatalf("Could not parse --tls-config. Must have exactly two components, separated by a comma")
	}
	ctx.Infof("Starting HTTP server with TLS using certificate [%s] and private key [%s] at https://%s", paths[0], paths[1], addr)
	if err := http.ListenAndServeTLS(addr, paths[0], paths[1], nil); err != nil {
		ctx.Fatalf("http.ListenAndServeTLS failed: %v", err)
	}
}

func httpAddress(externalHTTPAddr, httpAddr string) string {
	// If an externalHTTPAddr is provided use that.
	if externalHTTPAddr != "" {
		httpAddr = externalHTTPAddr
	}
	return fmt.Sprintf("https://%v", httpAddr)
}
