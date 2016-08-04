// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package global

import (
	"errors"

	"v.io/v23/discovery"
)

// validateAd returns an error if ad is not suitable for advertising.
func validateAd(ad *discovery.Advertisement) error {
	if !ad.Id.IsValid() {
		return errors.New("id not valid")
	}
	if len(ad.InterfaceName) == 0 {
		return errors.New("interface name not provided")
	}
	if len(ad.Addresses) == 0 {
		return errors.New("address not provided")
	}
	if len(ad.Attachments) > 0 {
		return errors.New("attachments not supported")
	}
	return nil
}
