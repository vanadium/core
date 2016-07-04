// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package signer provides access to the .
//
// Usage example:
//
//   import "google.golang.org/api/signer/v1"
//   ...
//   signerService, err := signer.New(oauthHttpClient)
package signer

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/net/context"
	"google.golang.org/api/googleapi"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// Always reference these packages, just in case the auto-generated code
// below doesn't.
var _ = bytes.NewBuffer
var _ = strconv.Itoa
var _ = fmt.Sprintf
var _ = json.NewDecoder
var _ = io.Copy
var _ = url.Parse
var _ = googleapi.Version
var _ = errors.New
var _ = strings.Replace
var _ = context.Background

const apiId = "signer:v1"
const apiName = "signer"
const apiVersion = "v1"
const basePath = "https://vanadium-keystore.appspot.com/_ah/api/signer/v1/"

// OAuth2 scopes used by this API.
const (
	// View your email address
	UserinfoEmailScope = "https://www.googleapis.com/auth/userinfo.email"
)

func New(client *http.Client) (*Service, error) {
	if client == nil {
		return nil, errors.New("client is nil")
	}
	s := &Service{client: client, BasePath: basePath}
	return s, nil
}

type Service struct {
	client   *http.Client
	BasePath string // API endpoint base URL
}

type PublicKey struct {
	Base64 string `json:"base64,omitempty"`
}

type VSignature struct {
	R string `json:"r,omitempty"`

	S string `json:"s,omitempty"`
}

// method id "signer.publicKey":

type PublicKeyCall struct {
	s    *Service
	opt_ map[string]interface{}
}

// PublicKey:
func (s *Service) PublicKey() *PublicKeyCall {
	c := &PublicKeyCall{s: s, opt_: make(map[string]interface{})}
	return c
}

// Fields allows partial responses to be retrieved.
// See https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *PublicKeyCall) Fields(s ...googleapi.Field) *PublicKeyCall {
	c.opt_["fields"] = googleapi.CombineFields(s)
	return c
}

func (c *PublicKeyCall) Do() (*PublicKey, error) {
	var body io.Reader = nil
	params := make(url.Values)
	params.Set("alt", "json")
	if v, ok := c.opt_["fields"]; ok {
		params.Set("fields", fmt.Sprintf("%v", v))
	}
	urls := googleapi.ResolveRelative(c.s.BasePath, "publicKey")
	urls += "?" + params.Encode()
	req, _ := http.NewRequest("POST", urls, body)
	googleapi.SetOpaque(req.URL)
	req.Header.Set("User-Agent", "google-api-go-client/0.5")
	res, err := c.s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	var ret *PublicKey
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "POST",
	//   "id": "signer.publicKey",
	//   "path": "publicKey",
	//   "response": {
	//     "$ref": "PublicKey"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/userinfo.email"
	//   ]
	// }

}

// method id "signer.sign":

type SignCall struct {
	s      *Service
	base64 string
	opt_   map[string]interface{}
}

// Sign:
func (s *Service) Sign(base64 string) *SignCall {
	c := &SignCall{s: s, opt_: make(map[string]interface{})}
	c.base64 = base64
	return c
}

// Fields allows partial responses to be retrieved.
// See https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *SignCall) Fields(s ...googleapi.Field) *SignCall {
	c.opt_["fields"] = googleapi.CombineFields(s)
	return c
}

func (c *SignCall) Do() (*VSignature, error) {
	var body io.Reader = nil
	params := make(url.Values)
	params.Set("alt", "json")
	if v, ok := c.opt_["fields"]; ok {
		params.Set("fields", fmt.Sprintf("%v", v))
	}
	urls := googleapi.ResolveRelative(c.s.BasePath, "sign/{base64}")
	urls += "?" + params.Encode()
	req, _ := http.NewRequest("POST", urls, body)
	googleapi.Expand(req.URL, map[string]string{
		"base64": c.base64,
	})
	req.Header.Set("User-Agent", "google-api-go-client/0.5")
	res, err := c.s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	var ret *VSignature
	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "httpMethod": "POST",
	//   "id": "signer.sign",
	//   "parameterOrder": [
	//     "base64"
	//   ],
	//   "parameters": {
	//     "base64": {
	//       "location": "path",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "sign/{base64}",
	//   "response": {
	//     "$ref": "VSignature"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/userinfo.email"
	//   ]
	// }

}
