// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package templates

import "html/template"

var SelectCaveats = template.Must(selectCaveats.Parse(headPartial))

var selectCaveats = template.Must(template.New("bless").Parse(`<!doctype html>
<html>
<head>
  <title>Add Blessing - Vanadium Identity Provider</title>

  {{template "head" .}}

</head>

<body class="identityprovider-layout">

  <header>
    <nav class="left">
      <a href="#" class="logo">Vanadium</a>
      <span class="service-name">Identity Provider</span>
    </nav>
    <nav class="right">
      <a href="#">{{.BlessingName}}</a>
    </nav>
  </header>

  <main class="add-blessing">

    <form method="POST" id="caveats-form" name="input"
    action="{{.MacaroonURL}}" role="form" novalidate>
      <input type="text" class="hidden" name="macaroon" value="{{.Macaroon}}">

      <h1 class="page-head">Add blessing</h1>
      <p>
        This blessing allows the Vanadium Identity Provider to authorize your
        application's credentials and provides your application access to the
        data associated with your Google Account. Blessing names contain the
        email address associated with your Google Account, and will be visible
        to peers you connect to.
      </p>

      <div class="note">
        <p>
          <strong>
            Using Vanadium in production applications is discouraged at this
          time.</strong><br>
          During this preview, the
          <a href="https://vanadium.github.io/glossary.html#blessing-root" target="_">
            blessing root
          </a>
          may change without notice.
        </p>
      </div>

      <label for="blessingExtension">Blessing name</label>
      <div class="value">
        {{.BlessingName}}:
        <input name="blessingExtension" type="text" placeholder="extension">
        <input type="hidden" id="timezoneOffset" name="timezoneOffset">
      </div>

      <label>Caveats</label>
      <div class="caveatRow">
        <div class="define-caveat">
          <span class="selected value RevocationCaveatSelected">
            Active until revoked
          </span>
          <span class="selected value ExpiryCaveatSelected hidden">
            Expires on
          </span>
          <span class="selected value MethodCaveatSelected hidden">
            Allowed methods are
          </span>
          <span class="selected value PeerBlessingsCaveatSelected hidden">
            Allowed peers are
          </span>

          <select name="caveat" class="caveats hidden">
            <option name="RevocationCaveat" value="RevocationCaveat"
            class="cavOption">Active until revoked</option>
            <option name="ExpiryCaveat" value="ExpiryCaveat"
            class="cavOption">Expires on</option>
            <option name="MethodCaveat" value="MethodCaveat"
            class="cavOption">Allowed methods are</option>
            <option name="PeerBlessingsCaveat" value="PeerBlessingsCaveat"
            class="cavOption">Allowed peers are</option>
          </select>

          <input type="text" class="caveatInput hidden"
            id="RevocationCaveat" name="RevocationCaveat">
          <input type="datetime-local" class="caveatInput expiry hidden"
            id="ExpiryCaveat" name="ExpiryCaveat">
          <input type="text" class="caveatInput hidden"
           id="MethodCaveat" name="MethodCaveat"
           placeholder="comma-separated method list">
          <input type="text" class="caveatInput hidden"
            id="PeerBlessingsCaveat" name="PeerBlessingsCaveat"
            placeholder="comma-separated blessing list">
        </div>
        <div class="add-caveat">
          <a href="#" class="addMore">Add more caveats</a>
        </div>
      </div>

      <div class="action-buttons">
        <input type="text" class="hidden" name="cancelled" id="cancelled" value="false">
        <button class="button-tertiary" id="cancel" type="button">Cancel</button>
        <button class="button-primary" type="submit" id="bless">Bless</button>
      </div>

      <p class="disclaimer-text">
        By clicking "Bless", you agree to the Google
        <a href="https://www.google.com/intl/en/policies/terms/">General Terms of Service</a>,
        <a href="https://developers.google.com/terms/">APIs Terms of Service</a>,
        and <a href="https://www.google.com/intl/en/policies/privacy/">Privacy Policy</a>
      </p>
    </form>
  </main>

  <script src="{{.AssetsPrefix}}/identity/moment.js"></script>
  <script src="{{.AssetsPrefix}}/identity/jquery.js"></script>

  <script>
  $(document).ready(function() {
    var numCaveats = 1;
    // When a caveat selector changes show the corresponding input box.
    $('body').on('change', '.caveats', function (){
      var caveatSelector = $(this).parents('.caveatRow');

      // Hide the visible inputs and show the selected one.
      caveatSelector.find('.caveatInput').hide();
      var caveatName = $(this).val();
      if (caveatName !== 'RevocationCaveat') {
        caveatSelector.find('#'+caveatName).show();
      }
    });

    var updateNewSelector = function(newSelector, caveatName) {
      // disable the option from being selected again and make the next caveat the
      // default for the next selector.
      var selectedOption = newSelector.find('option[name="' + caveatName + '"]');
      selectedOption.prop('disabled', true);
      var newCaveat = newSelector.find('option:enabled').first();
      newCaveat.prop('selected', true);
      newSelector.find('.caveatInput').hide();
      newSelector.find('#'+newCaveat.attr('name')).show();
    }

    // Upon clicking the 'Add Caveat' button a new caveat selector should appear.
    $('body').on('click', '.addCaveat', function() {
      var selector = $(this).parents('.caveatRow');
      var newSelector = selector.clone();
      var caveatName = selector.find('.caveats').val();

      updateNewSelector(newSelector, caveatName);

      // Change the selector's select to a fixed label and fix the inputs.
      selector.find('.caveats').hide();
      selector.find('.'+caveatName+'Selected').show();
      selector.find('.caveatInput').prop('readonly', true);

      selector.after(newSelector);
      $(this).replaceWith('<button type="button" class="button-passive right removeCaveat hidden">Remove</button>');

      numCaveats += 1;
      if (numCaveats > 1) {
        $('.removeCaveat').show();
      }
      if (numCaveats >= 4) {
        $('.addCaveat').hide();
        $('.caveats').hide();
      }
    });

    // If add more is selected, remove the button and show the caveats selector.
    $('body').on('click', '.addMore', function() {
      var selector = $(this).parents('.caveatRow');
      var newSelector = selector.clone();
      var caveatName = selector.find('.caveats').val();

      updateNewSelector(newSelector, caveatName);

      newSelector.find('.caveats').show();
      // Change the 'Add more' button in the copied selector to an 'Add caveats' button.
      newSelector.find('.addMore').replaceWith('<button type="button" class="button-primary right addCaveat">Add</button>');
      // Hide the default selected caveat for the copied selector.
      newSelector.find('.selected').hide();

      selector.after(newSelector);
      $(this).replaceWith('<button type="button" class="button-passive right removeCaveat hidden">Remove</button>');
    });

    // Upon clicking bless, caveats that have not been added yet should be removed,
    // before they are sent to the server.
    $('#bless').click(function(){
      $('.addCaveat').parents('.caveatRow').remove();
      $("#caveats-form").submit();
    });

    // Upon clicking the 'Remove Caveat' button, the caveat row should be removed.
    $('body').on('click', '.removeCaveat', function() {
      var selector = $(this).parents('.caveatRow')
      var caveatName = selector.find('.caveats').val();

      // Enable choosing this caveat again.
      $('option[name="' + caveatName + '"]').last().prop('disabled', false);

      selector.remove();

      numCaveats -= 1;
      if (numCaveats == 1) {
        $('.removeCaveat').hide();
      }
      $('.addCaveat').show();
      $('.caveats').last().show();
    });

    // Get the timezoneOffset for the server to create a correct expiry caveat.
    // The offset is the minutes between UTC and local time.
    var d = new Date();
    $('#timezoneOffset').val(d.getTimezoneOffset());

    // Set the datetime picker to have a default value of one day from now.
    $('.expiry').val(moment().add(1, 'd').format('YYYY-MM-DDTHH:mm'));
    // Remove the clear button from the date input.
    $('.expiry').attr('required', 'required');

    // Activate the cancel button.
    $('#cancel').click(function(){
      $("#cancelled").val("true");
      $("#caveats-form").submit();
    });
  });
  </script>
</body>
</html>`))
