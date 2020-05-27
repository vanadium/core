// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package httplib

import (
	"bytes"

	"golang.org/x/net/trace"

	"v.io/v23/context"
	"v.io/v23/rpc"
	v23_http "v.io/v23/services/http"
	"v.io/x/ref/services/http/httplib"
)

type httpService struct{}

func (f *httpService) RawDo(_ *context.T, _ rpc.ServerCall, req v23_http.Request) (
	data []byte, err error) {

	buf := bytes.NewBuffer(data)
	httpReq := httplib.HTTPRequestFromVDLRequest(req)

	trace.Render(buf, httpReq, false)

	return buf.Bytes(), nil
}

//nolint:golint // API change required.
func NewHttpService() interface{} {
	return v23_http.HttpServer(&httpService{})
}
