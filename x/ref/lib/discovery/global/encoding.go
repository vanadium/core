// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package global

import (
	"strconv"
	"strings"

	"v.io/v23/discovery"
	"v.io/v23/naming"
	"v.io/v23/vom"
)

// encodeAdToSuffix encodes the ad.Id and the ad.Attributes into the suffix at
// which we mount the advertisement.
// The format of the generated suffix is id/interfaceName/timestamp/attributes.
//
// TODO(suharshs): Currently Attachments are not encoded in global discovery.
func encodeAdToSuffix(ad *discovery.Advertisement, timestampNs int64) (string, error) {
	b, err := vom.Encode(ad.Attributes)
	if err != nil {
		return "", err
	}
	// Escape suffixDelim to use it as our delimeter between the id and the attrs.
	id := ad.Id.String()
	// InterfaceName can never be empty as per validate.go.
	interfaceName := naming.EncodeAsNameElement(ad.InterfaceName)
	timestamp := strconv.FormatInt(timestampNs, 10)
	attr := naming.EncodeAsNameElement(string(b))
	return naming.Join(id, interfaceName, timestamp, attr), nil
}

// decodeAdFromSuffix decodes in into an advertisement.
// The format of the input suffix is id/interfaceName/timestamp/attributes.
func decodeAdFromSuffix(in string) (*discovery.Advertisement, int64, error) {
	parts := strings.SplitN(in, "/", 4)
	if len(parts) != 4 {
		return nil, 0, ErrorfAdInvalidEncoding(nil, "ad (%v}) has invalid encoding", in)
	}
	var err error
	var ok bool
	ad := &discovery.Advertisement{}
	if ad.Id, err = discovery.ParseAdId(parts[0]); err != nil {
		return nil, 0, err
	}
	if ad.InterfaceName, ok = naming.DecodeFromNameElement(parts[1]); !ok {
		return nil, 0, ErrorfAdInvalidEncoding(nil, "ad (%v}) has invalid encoding", in)
	}
	timestampNs, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return nil, 0, err
	}
	attrs, ok := naming.DecodeFromNameElement(parts[3])
	if !ok {
		return nil, 0, ErrorfAdInvalidEncoding(nil, "ad (%v}) has invalid encoding", in)
	}
	if err = vom.Decode([]byte(attrs), &ad.Attributes); err != nil {
		return nil, 0, err
	}
	return ad, timestampNs, nil
}
