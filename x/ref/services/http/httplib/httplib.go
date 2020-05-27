// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package httplib

import (
	"bytes"
	"io/ioutil"
	"net/url"

	go_http "net/http"

	v23_http "v.io/v23/services/http"
)

func VDLRequestFromHTTPRequest(req *go_http.Request) v23_http.Request {
	url := v23_http.Url{
		Scheme:   req.URL.Scheme,
		Opaque:   req.URL.Opaque,
		Host:     req.URL.Host,
		Path:     req.URL.Path,
		RawPath:  req.URL.RawPath,
		RawQuery: req.URL.RawQuery,
		Fragment: req.URL.Fragment,
	}

	bodyBuf := new(bytes.Buffer)
	bodyBuf.ReadFrom(req.Body) //nolint:errcheck

	return v23_http.Request{
		Method:           req.Method,
		Url:              url,
		Proto:            req.Proto,
		ProtoMajor:       int16(req.ProtoMajor),
		ProtoMinor:       int16(req.ProtoMinor),
		Header:           req.Header,
		Body:             bodyBuf.Bytes(),
		ContentLength:    req.ContentLength,
		TransferEncoding: req.TransferEncoding,
		Close:            req.Close,
		Host:             req.Host,
		Form:             req.Form,
		PostForm:         req.PostForm,
		Trailer:          req.Trailer,
		RemoteAddr:       req.RemoteAddr,
		RequestUri:       req.RequestURI,
	}
}

func HTTPRequestFromVDLRequest(req v23_http.Request) *go_http.Request {
	url := &url.URL{
		Scheme:   req.Url.Scheme,
		Opaque:   req.Url.Opaque,
		User:     nil,
		Host:     req.Url.Host,
		Path:     req.Url.Path,
		RawPath:  req.Url.RawPath,
		RawQuery: req.Url.RawQuery,
		Fragment: req.Url.Fragment,
	}

	return &go_http.Request{
		Method:           req.Method,
		URL:              url,
		Proto:            req.Proto,
		ProtoMajor:       int(req.ProtoMajor),
		ProtoMinor:       int(req.ProtoMinor),
		Header:           req.Header,
		Body:             ioutil.NopCloser(bytes.NewReader(req.Body)),
		ContentLength:    req.ContentLength,
		TransferEncoding: req.TransferEncoding,
		Close:            req.Close,
		Host:             req.Host,
		Form:             req.Form,
		PostForm:         req.PostForm,
		MultipartForm:    nil,
		Trailer:          req.Trailer,
		RemoteAddr:       req.RemoteAddr,
		RequestURI:       req.RequestUri,
	}
}
