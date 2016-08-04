// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package caveats

import (
	"net/http"
)

// CaveatSelector is used to render a web page where the user can select caveats
// to be added to a blessing being granted
type CaveatSelector interface {
	// Render renders the caveat input form. When the user has completed inputing caveats,
	// Render should redirect to the specified redirect route.
	// blessingName is the name used for the blessings that is being caveated.
	// state is any state passed by the caller (e.g., for CSRF mitigation) and is returned by ParseSelections.
	// redirectRoute is the route to be returned to.
	Render(blessingName, state, redirectURL string, w http.ResponseWriter, r *http.Request) error
	// ParseSelections parse the users choices of Caveats, and returns the information needed to create them,
	// the state passed to Render, and any additionalExtension selected by the user to further extend the blessing.
	ParseSelections(r *http.Request) (caveats []CaveatInfo, state string, additionalExtension string, err error)
}
