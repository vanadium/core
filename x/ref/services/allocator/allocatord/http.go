// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"v.io/v23/context"
	"v.io/v23/security"
	"v.io/x/ref/services/debug/debug/browseserver"
)

const (
	routeRoot      = "/"
	routeHome      = "/home"
	routeCreate    = "/create"
	routeDashboard = "/dashboard"
	routeDebug     = "/debug"
	routeDestroy   = "/destroy"
	routeSuspend   = "/suspend"
	routeResume    = "/resume"
	routeReset     = "/reset"
	routeOauth     = "/oauth2"
	routeStatic    = "/static/"
	routeStats     = "/stats"
	routeHealth    = "/health"

	paramMessage = "message"
	paramHMAC    = "hmac"
	// The following parameter names are hardcorded in static/dash.js, and
	// should be changed in tandem.
	paramInstance         = "instance"
	paramDashbordDuration = "d"
	// paramMountName has to match the name parameter in debug browser.
	paramMountName = "n"

	cookieValidity = 7 * 24 * time.Hour
)

type param struct {
	key, value string
}

type params map[string]string

func makeURL(ctx *context.T, baseURL string, p params) string {
	u, err := url.Parse(baseURL)
	if err != nil {
		ctx.Errorf("Parse url error for %v: %v", baseURL, err)
		return ""
	}
	v := url.Values{}
	for param, value := range p {
		v.Add(param, value)
	}
	u.RawQuery = v.Encode()
	return u.String()
}

func replaceParam(ctx *context.T, origURL, param, value string) string {
	u, err := url.Parse(origURL)
	if err != nil {
		ctx.Errorf("Parse url error for %v: %v", origURL, err)
		return ""
	}
	v := u.Query()
	v.Set(param, value)
	u.RawQuery = v.Encode()
	return u.String()
}

type httpArgs struct {
	addr,
	externalURL,
	serverName,
	dashboardGCMMetric,
	dashboardGCMProject,
	monitoringKeyFile string
	secureCookies     bool
	oauthCreds        *oauthCredentials
	baseBlessings     security.Blessings
	baseBlessingNames []string
	// URI prefix for static assets served from (another) content server.
	staticAssetsPrefix string
	// Manages locally served resources.
	assets *assetsHelper
}

func (a httpArgs) validate() error {
	switch {
	case a.addr == "":
		return errors.New("addr is empty")
	case a.externalURL == "":
		return errors.New("externalURL is empty")
	}
	if err := a.oauthCreds.validate(); err != nil {
		return fmt.Errorf("oauth creds invalid: %v", err)
	}
	return nil
}

// startHTTP is the entry point to the http interface.  It configures and
// launches the http server, and returns a cleanup method to be called at
// shutdown time.
func startHTTP(ctx *context.T, args httpArgs) func() error {
	if err := args.validate(); err != nil {
		ctx.Fatalf("Invalid args %#v: %v", args, err)
	}
	baker := &signedCookieBaker{
		secure:   args.secureCookies,
		signKey:  args.oauthCreds.HashKey,
		validity: cookieValidity,
	}

	debugBrowserServeMux, err := browseserver.CreateServeMux(ctx, time.Second*10, false, "", routeDebug)
	if err != nil {
		ctx.Fatalf("Failed to setup debug browser handlers: %v", err)
	}

	// mutating should be true for handlers that mutate state.  For such
	// handlers, any re-authentication should result in redirection to the
	// home page (to foil CSRF attacks that trick the user into launching
	// actions with consequences).
	newHandler := func(f handlerFunc, mutating, forceLogin bool) *handler {
		return &handler{
			ss: &serverState{
				ctx:   ctx,
				args:  args,
				baker: baker,
			},
			f:          f,
			mutating:   mutating,
			forceLogin: forceLogin,
		}
	}

	http.HandleFunc(routeRoot, func(w http.ResponseWriter, r *http.Request) {
		tmplArgs := struct {
			AssetsPrefix,
			Home,
			Email,
			ServerName string
		}{
			AssetsPrefix: args.staticAssetsPrefix,
			Home:         routeHome,
			Email:        "", // Ask the user to log in.
			ServerName:   args.serverName,
		}
		if err := args.assets.executeTemplate(w, rootTmpl, tmplArgs); err != nil {
			args.assets.errorOccurred(ctx, w, r, routeHome, err)
			ctx.Infof("%s[%s] : error %v", r.Method, r.URL, err)
		}
	})
	http.Handle(routeHome, newHandler(handleHome, false, true))
	http.Handle(routeCreate, newHandler(handleCreate, true, true))
	http.Handle(routeDashboard, newHandler(handleDashboard, false, true))
	http.Handle(routeDebug+"/", newHandler(
		func(ss *serverState, rs *requestState) error {
			return handleDebug(ss, rs, debugBrowserServeMux)
		}, false, true))
	http.Handle(routeDestroy, newHandler(handleDestroy, true, true))
	http.Handle(routeSuspend, newHandler(handleSuspend, true, true))
	http.Handle(routeResume, newHandler(handleResume, true, true))
	http.Handle(routeReset, newHandler(handleReset, true, true))
	http.HandleFunc(routeOauth, func(w http.ResponseWriter, r *http.Request) {
		handleOauth(ctx, args, baker, w, r)
	})
	http.Handle(routeStatic, http.StripPrefix(routeStatic, args.assets))
	http.Handle(routeStats, newHandler(handleStats, false, false))
	http.HandleFunc(routeHealth, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	ln, err := net.Listen("tcp", args.addr)
	if err != nil {
		ctx.Fatalf("Listen failed: %v", err)
	}
	ctx.Infof("HTTP server at %v [%v]", ln.Addr(), args.externalURL)
	go func() {
		if err := http.Serve(ln, nil); err != nil {
			ctx.Fatalf("Serve failed: %v", err)
		}
	}()
	// NOTE(caprita): closing the listener is necessary but not sufficient
	// for graceful HTTP server shutdown (which should include draining
	// in-flight requests).  See https://github.com/facebookgo/httpdown for
	// example.
	return ln.Close
}

type serverState struct {
	ctx   *context.T
	args  httpArgs
	baker cookieBaker
}

type requestState struct {
	email, csrfToken string
	w                http.ResponseWriter
	r                *http.Request
}

type handlerFunc func(ss *serverState, rs *requestState) error

// handler wraps handler functions and takes care of providing them with a
// Vanadium context, configuration args, and user's email address (performing
// the oauth flow if the user is not logged in yet).
type handler struct {
	ss         *serverState
	f          handlerFunc
	mutating   bool
	forceLogin bool
}

// ServeHTTP verifies that the user is logged in.  If the user is logged in, it
// extracts the email address from the cookie and passes it to the handler
// function.  If the user is not logged in, and forceLogin is true, it redirects
// to the oauth flow; otherwise, it leaves the email field blank when invoking
// the handler function.
func (h handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := h.ss.ctx
	var (
		email, csrfToken, sessionBlurb string
		err                            error
	)
	if !h.forceLogin {
		if email, csrfToken, err = checkSession(h.ss.baker, r, h.mutating); err != nil {
			sessionBlurb = fmt.Sprintf("no session (%v)", err)
		}
	} else {
		oauthCfg := oauthConfig(h.ss.args.externalURL, h.ss.args.oauthCreds)
		if email, csrfToken, err = requireSession(ctx, oauthCfg, h.ss.baker, w, r, h.mutating); err != nil {
			h.ss.args.assets.errorOccurred(ctx, w, r, routeHome, err)
			ctx.Infof("%s[%s] : error %v", r.Method, r.URL, err)
			return
		}
		if email == "" {
			ctx.Infof("%s[%s] -> login", r.Method, r.URL)
			return
		}
	}
	rs := &requestState{
		email:     email,
		csrfToken: csrfToken,
		w:         w,
		r:         r,
	}
	if err := h.f(h.ss, rs); err != nil {
		h.ss.args.assets.errorOccurred(ctx, w, r, routeHome, err)
		ctx.Infof("%s[%s] %s : error %v", r.Method, r.URL, sessionBlurb, err)
		return
	}
	ctx.Infof("%s[%s] %s : OK", r.Method, r.URL, sessionBlurb)
}
