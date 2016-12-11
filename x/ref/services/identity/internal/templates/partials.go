// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package templates

var headPartial = `{{define "head"}}
  <meta
     name="viewport"
     content="width=device-width,
              initial-scale=1,
              maximum-scale=1,
              user-scalable=no,
              minimal-ui">
  <meta
     name="apple-mobile-web-app-capable"
     content="yes">

  <meta
     name="apple-mobile-web-app-status-bar-style"
     content="black">


  <link href='//fonts.googleapis.com/css?family=Source+Code+Pro:400,500|Roboto:500,400italic,300,500italic,300italic,400'
    rel='stylesheet'
    type='text/css'>

  <link rel="stylesheet" href="{{.AssetsPrefix}}/identity.css">

  <link rel="apple-touch-icon" sizes="57x57" href="{{.AssetsPrefix}}/favicons/apple-touch-icon-57x57.png">
  <link rel="apple-touch-icon" sizes="114x114" href="{{.AssetsPrefix}}/favicons/apple-touch-icon-114x114.png">
  <link rel="apple-touch-icon" sizes="72x72" href="{{.AssetsPrefix}}/favicons/apple-touch-icon-72x72.png">
  <link rel="apple-touch-icon" sizes="144x144" href="{{.AssetsPrefix}}/favicons/apple-touch-icon-144x144.png">
  <link rel="apple-touch-icon" sizes="60x60" href="{{.AssetsPrefix}}/favicons/apple-touch-icon-60x60.png">
  <link rel="apple-touch-icon" sizes="120x120" href="{{.AssetsPrefix}}/favicons/apple-touch-icon-120x120.png">
  <link rel="apple-touch-icon" sizes="76x76" href="{{.AssetsPrefix}}/favicons/apple-touch-icon-76x76.png">
  <link rel="apple-touch-icon" sizes="152x152" href="{{.AssetsPrefix}}/favicons/apple-touch-icon-152x152.png">
  <link rel="apple-touch-icon" sizes="180x180" href="{{.AssetsPrefix}}/favicons/apple-touch-icon-180x180.png">
  <link rel="icon" type="image/png" href="{{.AssetsPrefix}}/favicons/favicon-192x192.png" sizes="192x192">
  <link rel="icon" type="image/png" href="{{.AssetsPrefix}}/favicons/favicon-160x160.png" sizes="160x160">
  <link rel="icon" type="image/png" href="{{.AssetsPrefix}}/favicons/favicon-96x96.png" sizes="96x96">
  <link rel="icon" type="image/png" href="{{.AssetsPrefix}}/favicons/favicon-16x16.png" sizes="16x16">
  <link rel="icon" type="image/png" href="{{.AssetsPrefix}}/favicons/favicon-32x32.png" sizes="32x32">
  <meta name="msapplication-TileColor" content="#da532c">
  <meta name="msapplication-TileImage" content="{{.AssetsPrefix}}/favicons/mstile-144x144.png">
{{end}}`

var headerPartial = `{{define "header"}}
  <header>
    <nav class="left">
      <a href="#" class="logo">Vanadium</a>
      <span class="service-name">Identity Provider</span>
    </nav>
    <nav class="right">
      {{if .Email}}
        <a href="#">{{.Email}}</a>
      {{end}}
    </nav>
  </header>
{{end}}`

var sidebarPartial = `{{define "sidebar"}}
<section class="provider-info">
  <div class="provider-info-section">
    <h5>Root name</h5>
    <span class="provider-address">
      {{.Self}}
    </span>

    <h5>Public key</h5>
    <span class="provider-address">
      {{.Self.PublicKey}}
    </span>

    <p>
      Get this provider’s root name and public key as a <a
      href="http://en.wikipedia.org/wiki/X.690#DER_encoding" target="_blank">
      DER</a>-encoded <a href="/auth/blessing-root" target="_blank">
      JSON object</a>.
    </p>
  </div>

  {{if .DischargeServers}}
    <div class="provider-info-section">
      <h5>Discharges</h5>
      <p>
        Provided via Vanadium RPC to:
        <span class="provider-address">
          {{range .DischargeServers}}{{.}}{{end}}
        </span>
      </p>
    </div>
  {{end}}

  <div class="provider-info-section">
    <h5>Learn more</h5>
    <p>
    Vanadium Concepts: <a href="https://vanadium.github.io/concepts/security.html">Security</a><br>
    <a href="https://vanadium.github.io/tools/identity-service-faq.html">FAQ</a><br>
    </p>
  </div>
</section>
{{end}}`
