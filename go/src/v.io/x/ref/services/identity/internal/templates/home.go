// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package templates

import "html/template"

var Home = template.Must(homeWithHeader.Parse(sidebarPartial))
var homeWithHead = template.Must(home.Parse(headPartial))
var homeWithHeader = template.Must(homeWithHead.Parse(headerPartial))

var home = template.Must(template.New("main").Parse(`<!doctype html>
<html>
<head>
  <title>Vanadium Identity Provider</title>
  {{template "head" .}}
</head>

<body class="identityprovider-layout">
  {{template "header" .}}
  <main>
    <h1 class="page-head">Authorize Vanadium apps with Google</h1>
    <p>
      The Vanadium Identity Provider authorizes Vanadium blessings based on your Google Account.<br>
      <a href="https://vanadium.github.io/glossary.html#identity-provider">Learn more</a>
    </p>
    <p>
      <a href="/auth/google/{{.ListBlessingsRoute}}" class="button-passive">
        Show blessings
      </a>
    </p>
    {{template "sidebar" .}}
  </main>
</body>
</html>`))
