// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package templates

import "html/template"

var ListBlessings = template.Must(listWithHeader.Parse(sidebarPartial))
var listWithHead = template.Must(listBlessings.Parse(headPartial))
var listWithHeader = template.Must(listWithHead.Parse(headerPartial))

var listBlessings = template.Must(template.New("auditor").Parse(`<!doctype html>
<html>
<head>
  <title>Blessings for {{.Email}} - Vanadium Identity Provider</title>
  {{template "head" .}}
  <link rel="stylesheet" href="{{.AssetsPrefix}}/identity/toastr.css">
</head>

<body class="identityprovider-layout">
  {{template "header" .}}
  <main>
    <h1 class="page-head">Authorize Vanadium apps with Google</h1>
    <p>
      The Vanadium Identity Provider authorizes Vanadium blessings based on your Google Account.<br>
      <a href="https://vanadium.github.io/glossary.html#identity-provider">Learn more</a>
    </p>

    <div class="blessings-list">
      <div class="blessings-header">
          <h1>Your blessings</h1>
          <h5>Issued</h5>
          <h5>Revoked</h5>
        </div>
    {{range .Log}}
      {{if .Error}}
        <h1>Error</h1>
        <p>
          Failed to read audit log.<br>
          {{.Error}}
        </p>
      {{else}} {{/* if .Error */}}
        <div class="blessings-item">
          <div class="blessing-details">
            <h3>{{.Blessed}}</h3>
            <p>
              <b>Public Key</b><br>
              {{.Blessed.PublicKey}}
            </p>
            <p class="blessing-caveats">
              <b>Caveats</b><br>
              {{range .Caveats}}
                {{.}}<br>
              {{end}}
            </p>
          </div>

          <div class="blessing-issued unixtime" data-unixtime={{.Timestamp.Unix}}>{{.Timestamp.String}}</div>

          <div class="blessing-revoked">
            {{if .Token}}
              <button class="revoke button-passive" value="{{.Token}}">Revoke</button>
            {{else if not .RevocationTime.IsZero}}
              <p class="unixtime" data-unixtime={{.RevocationTime.Unix}}>{{.RevocationTime.String}}</p>
            {{end}}
          </div>
        </div>
      {{end}} {{/* if .Error */}}
    {{else}} {{/* range .Log */}}
      <p>
        <a href="https://vanadium.github.io/installation/">Install Vanadium</a> to set up your first blessing.
      </p>
    {{end}} {{/* range .Log */}}
    </div>
  {{template "sidebar" .}}
  </main>

  <script src="{{.AssetsPrefix}}/identity/toastr.js"></script>
  <script src="{{.AssetsPrefix}}/identity/moment.js"></script>
  <script src="{{.AssetsPrefix}}/identity/jquery.js"></script>
  <script>
  function setTimeText(elem) {
    var timestamp = elem.data("unixtime");
    var m = moment(timestamp*1000.0);
    var style = elem.data("style");
    if (style === "absolute") {
      elem.html("<a href='#'>" + m.format("MMM DD, YYYY h:mm:ss a") + "</a>");
      elem.data("style", "fromNow");
    } else {
      elem.html("<a href='#'>" + m.fromNow() + "</a>");
      elem.data("style", "absolute");
    }
  }

  $(document).ready(function() {
    $(".unixtime").each(function() {
      // clicking the timestamp should toggle the display format.
      $(this).click(function() { setTimeText($(this)); });
      setTimeText($(this));
    });

    // Setup the revoke buttons click events.
    $(".revoke").click(function() {
      var revokeButton = $(this);
      $.ajax({
        url: "/auth/google/{{.RevokeRoute}}",
        type: "POST",
        data: JSON.stringify({
          "Token": revokeButton.val()
        })
      }).done(function(data) {
        if (data.success == "false") {
          failMessage(revokeButton);
          return;
        }
        revokeButton.replaceWith("<div>Revoked just now</div>");
      }).fail(function(xhr, textStatus){
        failMessage(revokeButton);
        console.error('Bad request: %s', status, xhr)
      });
    });
  });

  function failMessage(revokeButton) {
    revokeButton.parent().parent().fadeIn(function(){
      $(this).addClass("bg-danger");
    });
    toastr.options.closeButton = true;
    toastr.error('Unable to revoke identity', 'Error')
  }
  </script>

</body>
</html>`))
