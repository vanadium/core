// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package oauth implements an http.Handler that has two main purposes
// listed below:
//
// (1) Uses OAuth to authenticate and then renders a page that
//     displays all the blessings that were provided for that Google user.
//     The client calls the /listblessings route which redirects to listblessingscallback which
//     renders the list.
// (2) Performs the oauth flow for seeking a blessing using the principal tool
//     located at v.io/x/ref/cmd/principal.
//     The seek blessing flow works as follows:
//     (a) Client (principal tool) hits the /seekblessings route.
//     (b) /seekblessings performs oauth with a redirect to /seekblessingscallback.
//     (c) Client specifies desired caveats in the form that /seekblessingscallback displays.
//     (d) Submission of the form sends caveat information to /sendmacaroon.
//     (e) /sendmacaroon sends a macaroon with blessing information to client
//         (via a redirect to an HTTP server run by the tool).
//     (f) Client invokes bless rpc with macaroon.
package oauth

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/security"
	"v.io/v23/vom"
	"v.io/x/ref/services/identity/internal/auditor"
	"v.io/x/ref/services/identity/internal/caveats"
	"v.io/x/ref/services/identity/internal/revocation"
	"v.io/x/ref/services/identity/internal/templates"
	"v.io/x/ref/services/identity/internal/util"
)

const (
	clientIDCookie = "VeyronHTTPIdentityClientID"

	ListBlessingsRoute         = "listblessings"
	listBlessingsCallbackRoute = "listblessingscallback"
	revokeRoute                = "revoke"
	SeekBlessingsRoute         = "seekblessings"
	addCaveatsRoute            = "addcaveats"
	sendMacaroonRoute          = "sendmacaroon"
)

type HandlerArgs struct {
	// The principal to use.
	Principal security.Principal
	// The Key that is used for creating and verifying macaroons.
	// This needs to be common between the handler and the MacaroonBlesser service.
	MacaroonKey []byte
	// URL at which the hander is installed.
	// e.g. http://host:port/google/
	Addr string
	// BlessingLogReder is needed for reading audit logs.
	BlessingLogReader auditor.BlessingLogReader
	// The RevocationManager is used to revoke blessings granted with a revocation caveat.
	// If nil, then revocation caveats cannot be added to blessings and an expiration caveat
	// will be used instead.
	RevocationManager revocation.RevocationManager
	// The object name of the discharger service.
	DischargerLocation string
	// MacaroonBlessingService is a function that returns the object names to which macaroons
	// created by this HTTP handler can be exchanged for a blessing.
	MacaroonBlessingService func() []string
	// OAuthProvider is used to authenticate and get a blessee email.
	OAuthProvider OAuthProvider
	// CaveatSelector is used to obtain caveats from the user when seeking a blessing.
	CaveatSelector caveats.CaveatSelector
	// AssetsPrefix is the host where web assets for rendering the list blessings template are stored.
	AssetsPrefix string
	// DischargeServers is the list of published disharges services.
	DischargeServers []string
}

// BlessingMacaroon contains the data that is encoded into the macaroon for creating blessings.
type BlessingMacaroon struct {
	Creation  time.Time
	Caveats   []security.Caveat
	Name      string
	PublicKey []byte // Marshaled public key of the principal tool.
}

func redirectURL(baseURL, suffix string) string {
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}
	return baseURL + suffix
}

// NewHandler returns an http.Handler that expects to be rooted at args.Addr
// and can be used to authenticate with args.OAuthProvider, mint a new
// identity and bless it with the OAuthProvider email address.
func NewHandler(ctx *context.T, args HandlerArgs) http.Handler {
	csrfCop := util.NewCSRFCop(ctx)
	return &handler{
		args:    args,
		csrfCop: csrfCop,
		ctx:     ctx,
	}
}

type handler struct {
	args    HandlerArgs
	csrfCop *util.CSRFCop
	ctx     *context.T
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch path.Base(r.URL.Path) {
	case ListBlessingsRoute:
		h.listBlessings(h.ctx, w, r)
	case listBlessingsCallbackRoute:
		h.listBlessingsCallback(h.ctx, w, r)
	case revokeRoute:
		h.revoke(h.ctx, w, r)
	case SeekBlessingsRoute:
		h.seekBlessings(h.ctx, w, r)
	case addCaveatsRoute:
		h.addCaveats(h.ctx, w, r)
	case sendMacaroonRoute:
		h.sendMacaroon(h.ctx, w, r)
	default:
		util.HTTPBadRequest(w, r, nil)
	}
}

func (h *handler) listBlessings(ctx *context.T, w http.ResponseWriter, r *http.Request) {
	csrf, err := h.csrfCop.NewToken(w, r, clientIDCookie, nil)
	if err != nil {
		ctx.Infof("Failed to create CSRF token[%v] for request %#v", err, r)
		util.HTTPServerError(w, fmt.Errorf("failed to create new token: %v", err))
		return
	}
	http.Redirect(w, r, h.args.OAuthProvider.AuthURL(redirectURL(h.args.Addr, listBlessingsCallbackRoute), csrf, ReuseApproval), http.StatusFound)
}

func (h *handler) listBlessingsCallback(ctx *context.T, w http.ResponseWriter, r *http.Request) {
	if err := h.csrfCop.ValidateToken(r.FormValue("state"), r, clientIDCookie, nil); err != nil {
		ctx.Infof("Invalid CSRF token: %v in request: %#v", err, r)
		util.HTTPBadRequest(w, r, fmt.Errorf("Suspected request forgery: %v", err))
		return
	}
	email, err := h.args.OAuthProvider.ExchangeAuthCodeForEmail(r.FormValue("code"), redirectURL(h.args.Addr, listBlessingsCallbackRoute))
	if err != nil {
		util.HTTPBadRequest(w, r, err)
		return
	}

	type tmplentry struct {
		Timestamp      time.Time
		Caveats        []string
		RevocationTime time.Time
		Blessed        security.Blessings
		Token          string
		Error          error
	}
	self, _ := h.args.Principal.BlessingStore().Default()
	tmplargs := struct {
		Log                              chan tmplentry
		Email, RevokeRoute, AssetsPrefix string
		Self                             security.Blessings
		DischargeServers                 []string
	}{
		Log:              make(chan tmplentry),
		Email:            email,
		RevokeRoute:      revokeRoute,
		AssetsPrefix:     h.args.AssetsPrefix,
		Self:             self,
		DischargeServers: h.args.DischargeServers,
	}
	entrych := h.args.BlessingLogReader.Read(ctx, email)

	w.Header().Set("Context-Type", "text/html")
	// This MaybeSetCookie call is needed to ensure that a cookie is created. Since the
	// header cannot be changed once the body is written to, this needs to be called first.
	if _, err = h.csrfCop.MaybeSetCookie(w, r, clientIDCookie); err != nil {
		ctx.Infof("Failed to set CSRF cookie[%v] for request %#v", err, r)
		util.HTTPServerError(w, err)
		return
	}
	go func(ch chan tmplentry) {
		defer close(ch)
		for entry := range entrych {
			tmplEntry := tmplentry{
				Error:     entry.DecodeError,
				Timestamp: entry.Timestamp,
				Blessed:   entry.Blessings,
			}
			if len(entry.Caveats) > 0 {
				if tmplEntry.Caveats, err = prettyPrintCaveats(entry.Caveats); err != nil {
					ctx.Errorf("Failed to pretty print caveats: %v", err)
					tmplEntry.Error = fmt.Errorf("failed to pretty print caveats: %v", err)
				}
			}
			if len(entry.RevocationCaveatID) > 0 && h.args.RevocationManager != nil {
				if revocationTime := h.args.RevocationManager.GetRevocationTime(entry.RevocationCaveatID); revocationTime != nil {
					tmplEntry.RevocationTime = *revocationTime
				} else {
					caveatID := base64.URLEncoding.EncodeToString([]byte(entry.RevocationCaveatID))
					if tmplEntry.Token, err = h.csrfCop.NewToken(w, r, clientIDCookie, caveatID); err != nil {
						ctx.Errorf("Failed to create CSRF token[%v] for request %#v", err, r)
						tmplEntry.Error = fmt.Errorf("server error: unable to create revocation token")
					}
				}
			}
			ch <- tmplEntry
		}
	}(tmplargs.Log)
	if err := templates.ListBlessings.Execute(w, tmplargs); err != nil {
		ctx.Errorf("Unable to execute audit page template: %v", err)
		util.HTTPServerError(w, err)
	}
}

// prettyPrintCaveats returns a user friendly string for vanadium standard caveat.
// Unrecognized caveats will fall back to the Caveat's String() method.
func prettyPrintCaveats(cavs []security.Caveat) ([]string, error) {
	s := make([]string, len(cavs))
	for i, cav := range cavs {
		if cav.Id == security.PublicKeyThirdPartyCaveat.Id {
			c := cav.ThirdPartyDetails()
			s[i] = fmt.Sprintf("ThirdPartyCaveat: Requires discharge from %v (ID=%q)", c.Location(), c.ID())
			continue
		}

		var param interface{}
		if err := vom.Decode(cav.ParamVom, &param); err != nil {
			return nil, err
		}
		switch cav.Id {
		case security.ExpiryCaveat.Id:
			s[i] = fmt.Sprintf("Expires at %v", param)
		case security.MethodCaveat.Id:
			s[i] = fmt.Sprintf("Restricted to methods %v", param)
		case security.PeerBlessingsCaveat.Id:
			s[i] = fmt.Sprintf("Restricted to peers with blessings %v", param)
		default:
			s[i] = cav.String()
		}
	}
	return s, nil
}

func (h *handler) revoke(ctx *context.T, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	const (
		success = `{"success": "true"}`
		failure = `{"success": "false"}`
	)
	if h.args.RevocationManager == nil {
		ctx.Infof("no provided revocation manager")
		w.Write([]byte(failure)) //nolint:errcheck
		return
	}

	content, err := ioutil.ReadAll(r.Body)
	if err != nil {
		ctx.Infof("Failed to parse request: %s", err)
		w.Write([]byte(failure)) //nolint:errcheck
		return
	}
	var requestParams struct {
		Token string
	}
	if err := json.Unmarshal(content, &requestParams); err != nil {
		ctx.Infof("json.Unmarshal failed : %s", err)
		w.Write([]byte(failure)) //nolint:errcheck
		return
	}

	var caveatID string
	if caveatID, err = h.validateRevocationToken(ctx, requestParams.Token, r); err != nil {
		ctx.Infof("failed to validate token for caveat: %s", err)
		w.Write([]byte(failure)) //nolint:errcheck
		return
	}
	if err := h.args.RevocationManager.Revoke(caveatID); err != nil {
		ctx.Infof("Revocation failed: %s", err)
		w.Write([]byte(failure)) //nolint:errcheck
		return
	}

	w.Write([]byte(success)) //nolint:errcheck
}

func (h *handler) validateRevocationToken(ctx *context.T, token string, r *http.Request) (string, error) {
	var encCaveatID string
	if err := h.csrfCop.ValidateToken(token, r, clientIDCookie, &encCaveatID); err != nil {
		return "", fmt.Errorf("invalid CSRF token: %v in request: %#v", err, r)
	}
	caveatID, err := base64.URLEncoding.DecodeString(encCaveatID)
	if err != nil {
		return "", fmt.Errorf("decode caveatID failed: %v", err)
	}
	return string(caveatID), nil
}

type seekBlessingsMacaroon struct {
	RedirectURL, State string
	PublicKey          []byte // Marshaled public key of the principal tool.
}

func validLoopbackURL(u string) (*url.URL, error) {
	netURL, err := url.Parse(u)
	if err != nil {
		return nil, fmt.Errorf("invalid url: %v", err)
	}
	// Remove the port from the netURL.Host.
	host, _, _ := net.SplitHostPort(netURL.Host)
	// Check if its localhost or loopback ip
	if host == "localhost" {
		return netURL, nil
	}
	urlIP := net.ParseIP(host)
	if urlIP.IsLoopback() {
		return netURL, nil
	}
	return nil, fmt.Errorf("invalid loopback url")
}

func (h *handler) seekBlessings(ctx *context.T, w http.ResponseWriter, r *http.Request) {
	redirect := r.FormValue("redirect_url")
	if _, err := validLoopbackURL(redirect); err != nil {
		ctx.Infof("seekBlessings failed: invalid redirect_url: %v", err)
		util.HTTPBadRequest(w, r, fmt.Errorf("invalid redirect_url: %v", err))
		return
	}
	pubKeyBytes, err := base64.URLEncoding.DecodeString(r.FormValue("public_key"))
	if err != nil {
		ctx.Infof("seekBlessings failed: invalid public_key: %v", err)
		util.HTTPBadRequest(w, r, fmt.Errorf("invalid public_key: %v", err))
		return
	}
	outputMacaroon, err := h.csrfCop.NewToken(w, r, clientIDCookie, seekBlessingsMacaroon{
		RedirectURL: redirect,
		State:       r.FormValue("state"),
		PublicKey:   pubKeyBytes,
	})
	if err != nil {
		ctx.Infof("Failed to create CSRF token[%v] for request %#v", err, r)
		util.HTTPServerError(w, fmt.Errorf("failed to create new token: %v", err))
		return
	}
	http.Redirect(w, r, h.args.OAuthProvider.AuthURL(redirectURL(h.args.Addr, addCaveatsRoute), outputMacaroon, ExplicitApproval), http.StatusFound)
}

type addCaveatsMacaroon struct {
	ToolRedirectURL, ToolState, Email string
	ToolPublicKey                     []byte // Marshaled public key of the principal tool.
}

func (h *handler) addCaveats(ctx *context.T, w http.ResponseWriter, r *http.Request) {
	var inputMacaroon seekBlessingsMacaroon
	if err := h.csrfCop.ValidateToken(r.FormValue("state"), r, clientIDCookie, &inputMacaroon); err != nil {
		util.HTTPBadRequest(w, r, fmt.Errorf("Suspected request forgery: %v", err))
		return
	}
	email, err := h.args.OAuthProvider.ExchangeAuthCodeForEmail(r.FormValue("code"), redirectURL(h.args.Addr, addCaveatsRoute))
	if err != nil {
		util.HTTPBadRequest(w, r, err)
		return
	}
	outputMacaroon, err := h.csrfCop.NewToken(w, r, clientIDCookie, addCaveatsMacaroon{
		ToolRedirectURL: inputMacaroon.RedirectURL,
		ToolState:       inputMacaroon.State,
		ToolPublicKey:   inputMacaroon.PublicKey,
		Email:           email,
	})
	if err != nil {
		ctx.Infof("Failed to create caveatForm token[%v] for request %#v", err, r)
		util.HTTPServerError(w, fmt.Errorf("failed to create new token: %v", err))
		return
	}
	localBlessings := security.DefaultBlessingPatterns(h.args.Principal)
	if len(localBlessings) == 0 {
		ctx.Infof("server principal has no blessings: %v", h.args.Principal)
		util.HTTPServerError(w, fmt.Errorf("failed to get server blessings"))
		return
	}
	fullBlessingName := strings.Join([]string{string(localBlessings[0]), email}, security.ChainSeparator)
	if err := h.args.CaveatSelector.Render(fullBlessingName, outputMacaroon, redirectURL(h.args.Addr, sendMacaroonRoute), w, r); err != nil {
		ctx.Errorf("Unable to invoke render caveat selector: %v", err)
		util.HTTPServerError(w, err)
	}
}

func (h *handler) sendMacaroon(ctx *context.T, w http.ResponseWriter, r *http.Request) {
	var inputMacaroon addCaveatsMacaroon
	caveatInfos, macaroonString, blessingExtension, err := h.args.CaveatSelector.ParseSelections(r)
	cancelled := err == caveats.ErrSeekblessingsCancelled
	if !cancelled && err != nil {
		util.HTTPBadRequest(w, r, fmt.Errorf("failed to parse blessing information: %v", err))
		return
	}
	if err := h.csrfCop.ValidateToken(macaroonString, r, clientIDCookie, &inputMacaroon); err != nil {
		util.HTTPBadRequest(w, r, fmt.Errorf("suspected request forgery: %v", err))
		return
	}
	// Construct the url to send back to the tool.
	baseURL, err := validLoopbackURL(inputMacaroon.ToolRedirectURL)
	if err != nil {
		util.HTTPBadRequest(w, r, fmt.Errorf("invalid ToolRedirectURL: %v", err))
		return
	}
	// Now that we have a valid tool redirect url, we can send the errors to the tool.
	if cancelled {
		h.sendErrorToTool(ctx, w, r, inputMacaroon.ToolState, baseURL, caveats.ErrSeekblessingsCancelled)
	}
	caveats, err := h.caveats(ctx, caveatInfos)
	if err != nil {
		h.sendErrorToTool(ctx, w, r, inputMacaroon.ToolState, baseURL, fmt.Errorf("failed to create caveats: %v", err))
		return
	}
	parts := []string{inputMacaroon.Email}
	if len(blessingExtension) > 0 {
		parts = append(parts, blessingExtension)
	}
	if len(caveats) == 0 {
		h.sendErrorToTool(ctx, w, r, inputMacaroon.ToolState, baseURL, fmt.Errorf("server disallows attempts to bless with no caveats"))
		return
	}
	m := BlessingMacaroon{
		Creation:  time.Now(),
		Caveats:   caveats,
		Name:      strings.Join(parts, security.ChainSeparator),
		PublicKey: inputMacaroon.ToolPublicKey,
	}
	macBytes, err := vom.Encode(m)
	if err != nil {
		h.sendErrorToTool(ctx, w, r, inputMacaroon.ToolState, baseURL, fmt.Errorf("failed to encode BlessingsMacaroon: %v", err))
		return
	}
	marshalKey, err := h.args.Principal.PublicKey().MarshalBinary()
	if err != nil {
		h.sendErrorToTool(ctx, w, r, inputMacaroon.ToolState, baseURL, fmt.Errorf("failed to marshal public key: %v", err))
		return
	}
	encKey := base64.URLEncoding.EncodeToString(marshalKey)
	objectNames := h.args.MacaroonBlessingService()
	if len(objectNames) == 0 {
		h.sendErrorToTool(ctx, w, r, inputMacaroon.ToolState, baseURL, fmt.Errorf("failed to get local server endpoints"))
		return
	}
	params := url.Values{}
	mac, err := util.NewMacaroon(v23.GetPrincipal(ctx), macBytes)
	if err != nil {
		h.sendErrorToTool(ctx, w, r, inputMacaroon.ToolState, baseURL, err)
		return
	}
	params.Add("macaroon", string(mac))
	params.Add("state", inputMacaroon.ToolState)
	for _, s := range objectNames {
		params.Add("object_name", s)
	}
	params.Add("root_key", encKey)
	baseURL.RawQuery = params.Encode()
	http.Redirect(w, r, baseURL.String(), http.StatusFound)
}

func (h *handler) sendErrorToTool(ctx *context.T, w http.ResponseWriter, r *http.Request, toolState string, baseURL *url.URL, err error) {
	errEnc := base64.URLEncoding.EncodeToString([]byte(err.Error()))
	params := url.Values{}
	params.Add("error", errEnc)
	params.Add("state", toolState)
	baseURL.RawQuery = params.Encode()
	http.Redirect(w, r, baseURL.String(), http.StatusFound)
}

func (h *handler) caveats(ctx *context.T, caveatInfos []caveats.CaveatInfo) (cavs []security.Caveat, err error) {
	caveatFactories := caveats.NewCaveatFactory()
	for _, caveatInfo := range caveatInfos {
		if caveatInfo.Type == "Revocation" {
			caveatInfo.Args = []interface{}{h.args.RevocationManager, h.args.Principal.PublicKey(), h.args.DischargerLocation}
		}
		cav, err := caveatFactories.New(caveatInfo)
		if err != nil {
			return nil, err
		}
		cavs = append(cavs, cav)
	}
	return
}
