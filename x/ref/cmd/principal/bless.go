// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/options"
	"v.io/v23/security"
	"v.io/x/lib/vlog"
	"v.io/x/ref/services/identity"
)

type formResult struct {
	macaroon    string
	objectNames []string
	rootKey     string
}

func exchangeMacaroonForBlessing(ctx *context.T, macaroonChan <-chan formResult) (security.Blessings, error) {
	objectNames, macaroon, serviceKey, err := prepareBlessArgs(ctx, macaroonChan)
	if err != nil {
		return security.Blessings{}, err
	}

	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	me := &naming.MountEntry{}
	for _, n := range objectNames {
		var server string
		// me.Name is the suffix part of the address and should be the
		// same for all object names.
		server, me.Name = naming.SplitAddressName(n)
		me.Servers = append(me.Servers, naming.MountedServer{Server: server})
	}

	// Authorize the server by its public key (obtained from macaroonChan).
	//
	// objectNames is either:
	// - a list of pre-resolved names, or
	// - a single unresolved object name. (DEPRECATED)
	//
	// First we try the pre-resolved names. If the call fails, we try the
	// unresolved name, in which case, we must skip authorization during
	// name resolution because the blessings of the nameservers are not
	// rooted at recognized keys yet.
	blessings, err := identity.MacaroonBlesserClient("").Bless(
		ctx,
		macaroon,
		options.ServerAuthorizer{Authorizer: security.PublicKeyAuthorizer(serviceKey)},
		options.Preresolved{Resolution: me})
	if err != nil && len(objectNames) == 1 {
		// For backward compatibility, try to resolve the service name.
		blessings, err = identity.MacaroonBlesserClient(objectNames[0]).Bless(
			ctx,
			macaroon,
			options.ServerAuthorizer{Authorizer: security.PublicKeyAuthorizer(serviceKey)},
			options.NameResolutionAuthorizer{Authorizer: security.AllowEveryone()})
	}
	if err != nil {
		return blessings, fmt.Errorf("failed to get blessing from %q: %v", objectNames, err)
	}
	return blessings, nil
}

func prepareBlessArgs(ctx *context.T, macaroonChan <-chan formResult) (service []string, macaroon string, root security.PublicKey, err error) {
	result := <-macaroonChan

	marshalKey, err := base64.URLEncoding.DecodeString(result.rootKey)
	if err != nil {
		return nil, "", nil, fmt.Errorf("failed to decode root key: %v", err)
	}
	root, err = security.UnmarshalPublicKey(marshalKey)
	if err != nil {
		return nil, "", nil, fmt.Errorf("failed to unmarshal root key: %v", err)
	}

	return result.objectNames, result.macaroon, root, nil
}

func getMacaroonForBlessRPC(key security.PublicKey, blessServerURL string, blessedChan <-chan string, browser bool) (<-chan formResult, error) {
	// Setup a HTTP server to receive a blessing macaroon from the identity server.
	// Steps:
	// 1. Generate a state token to be included in the HTTP request
	//    (though, arguably, the random port assigment for the HTTP server is enough
	//    for XSRF protection)
	// 2. Setup a HTTP server which will receive the final blessing macaroon from the id server.
	// 3. Print out the link (to start the auth flow) for the user to click.
	// 4. Return the macaroon and the rpc object name(where to make the MacaroonBlesser.Bless RPC call)
	//    in the "result" channel.
	var stateBuf [32]byte
	if _, err := rand.Read(stateBuf[:]); err != nil {
		return nil, fmt.Errorf("failed to generate state token for OAuth: %v", err)
	}
	state := base64.URLEncoding.EncodeToString(stateBuf[:])

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("failed to setup authorization code interception server: %v", err)
	}
	result := make(chan formResult)

	redirectURL := fmt.Sprintf("http://%s/macaroon", ln.Addr())
	http.HandleFunc("/macaroon", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		tmplArgs := struct {
			Blessings string
			ErrShort  string
			ErrLong   string
			Browser   bool
		}{
			Browser: browser,
		}
		defer func() {
			if len(tmplArgs.ErrShort) > 0 {
				w.WriteHeader(http.StatusBadRequest)
			}
			if err := tmpl.Execute(w, tmplArgs); err != nil {
				vlog.Info("Failed to render template:", err)
			}
		}()

		defer close(result)
		if r.FormValue("state") != state {
			tmplArgs.ErrShort = "Unexpected request"
			tmplArgs.ErrLong = "Mismatched state parameter. Possible cross-site-request-forgery?"
			return
		}
		if errEnc := r.FormValue("error"); errEnc != "" {
			tmplArgs.ErrShort = "Failed to get blessings"
			errBytes, err := base64.URLEncoding.DecodeString(errEnc)
			if err != nil {
				tmplArgs.ErrLong = err.Error()
			} else {
				tmplArgs.ErrLong = string(errBytes)
			}
			return
		}
		result <- formResult{
			macaroon:    r.FormValue("macaroon"),
			objectNames: r.Form["object_name"],
			rootKey:     r.FormValue("root_key"),
		}

		blessed, ok := <-blessedChan
		if !ok {
			tmplArgs.ErrShort = "No blessings received"
			tmplArgs.ErrLong = "Unable to obtain blessings from the Vanadium service"
			return
		}
		tmplArgs.Blessings = blessed
		ln.Close()
	})
	go http.Serve(ln, nil) //nolint:errcheck

	// Print the link to start the flow.
	url, err := seekBlessingsURL(key, blessServerURL, redirectURL, state)
	if err != nil {
		return nil, fmt.Errorf("failed to create seekBlessingsURL: %s", err)
	}
	fmt.Fprintln(os.Stdout, "Please visit the following URL to seek blessings:")
	fmt.Fprintln(os.Stdout, url)
	// Make an attempt to start the browser as a convenience.
	// If it fails, doesn't matter - the client can see the URL printed above.
	// Use exec.Command().Start instead of exec.Command().Run since there is no
	// need to wait for the command to return (and indeed on some window managers,
	// the command will not exit until the browser is closed).
	if len(openCommand) != 0 && browser {
		exec.Command(openCommand, url).Start() //nolint:errcheck
	}
	return result, nil
}

func seekBlessingsURL(key security.PublicKey, blessServerURL, redirectURL, state string) (string, error) {
	baseURL, err := url.Parse(joinURL(blessServerURL, identity.SeekBlessingsRoute))
	if err != nil {
		return "", fmt.Errorf("failed to parse url: %v", err)
	}
	keyBytes, err := key.MarshalBinary()
	if err != nil {
		return "", fmt.Errorf("failed to marshal public key: %v", err)
	}
	params := url.Values{}
	params.Add("redirect_url", redirectURL)
	params.Add("state", state)
	params.Add("public_key", base64.URLEncoding.EncodeToString(keyBytes))
	baseURL.RawQuery = params.Encode()
	return baseURL.String(), nil
}

func joinURL(baseURL, suffix string) string {
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}
	return baseURL + suffix
}

var tmpl = template.Must(template.New("name").Parse(`<!doctype html>
<html>
<head>
<meta charset="UTF-8">
<!--Excluding any third-party hosted resources like scripts and stylesheets because otherwise we run the risk of leaking the macaroon out of this page (e.g., via the referrer header) -->
<title>Vanadium Identity: Google</title>
{{if and .Browser .Blessings}}
<!--Attempt to close the window. Though this script does not work on many browser configurations-->
<script type="text/javascript">window.close();</script>
{{end}}
</head>
<body>
<div>
{{if .ErrShort}}
<center><h1><span style="color:#FF6E40;">Error: </span>{{.ErrShort}}</h1></center>
<center><h2>{{.ErrLong}}</h2></center>
{{else}}
<center><h1>Received blessings: <tt>{{.Blessings}}</tt></h1></center>
<center><h2>You may close this tab now.</h2></center>
{{end}}
</div>
</body>
</html>`))
