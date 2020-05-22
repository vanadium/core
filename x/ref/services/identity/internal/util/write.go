// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package util

import (
	"bytes"
	"html/template"
	"net/http"

	"v.io/x/ref/internal/logger"
)

// HTTPBadRequest sends an HTTP 400 error on 'w' and renders a pretty page.
// If err is not nil, it also renders the string representation of err in the response page.
func HTTPBadRequest(w http.ResponseWriter, req *http.Request, err error) {
	w.WriteHeader(http.StatusBadRequest)
	if e := tmplBadRequest.Execute(w, badRequestData{Request: requestString(req), Error: err}); e != nil {
		logger.Global().Errorf("Failed to execute Bad Request Template: %v", e)
	}
}

// ServerError sends an HTTP 500 error on 'w' and renders a pretty page that
// also has the string representation of err.
func HTTPServerError(w http.ResponseWriter, err error) {
	w.WriteHeader(http.StatusInternalServerError)
	if e := tmplServerError.Execute(w, err); e != nil {
		logger.Global().Errorf("Failed to execute Server Error template: %v", e)
	}
}

var (
	tmplBadRequest = template.Must(template.New("Bad Request").Parse(`<!doctype html>
<html>
<head>
<meta charset="UTF8">
<title>Bad Request</title>
</head>
<body>
<h1>Bad Request</h1>
{{with $data := .}}
{{if $data.Error}}Error: {{$data.Error}}{{end}}
<pre>
{{$data.Request}}
</pre>
{{end}}
</body>
</html>`))

	tmplServerError = template.Must(template.New("Server Error").Parse(`<!doctype html>
<html>
<head>
<meta charset="UTF8">
<title>Server Error</title>
</head>
<body>
<h1>Oops! Error at the server</h1>
Error: {{.}}
<br/>
Ask the server administrator to check the server logs
</body>
</html>`))
)

func requestString(r *http.Request) string {
	var buf bytes.Buffer
	r.Write(&buf) //nolint:errcheck
	return buf.String()
}

type badRequestData struct {
	Request string
	Error   error
}
