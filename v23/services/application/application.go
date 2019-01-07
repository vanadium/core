// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Make the Envelope type JSON-codeable.

package application

import (
	"encoding/base64"
	"encoding/json"
	"time"

	"v.io/v23/security"
	"v.io/v23/verror"
	"v.io/v23/vom"
)

const pkgPath = "v.io/v23/services/application"

var (
	errCantVOMEncodePublisher    = verror.Register(pkgPath+".errCantVOMEncodePublisher", verror.NoRetry, "{1:}{2:} failed to vom-encode Publisher{:_}")
	errCantBase64DecodePublisher = verror.Register(pkgPath+".errCantBase64DecodePublisher", verror.NoRetry, "{1:}{2:} failed to base64-decode Publisher{:_}")
	errCantVOMDecodePublisher    = verror.Register(pkgPath+".errCantVOMDecodePublisher", verror.NoRetry, "{1:}{2:} failed to vom-decode Publisher{:_}")
)

type jsonType struct {
	Title             string
	Args              []string
	Binary            SignedFile
	Publisher         string // base64-vom-encoded security.Blessings
	Env               []string
	Packages          Packages
	Restarts          int32
	RestartTimeWindow time.Duration
}

func (env Envelope) MarshalJSON() ([]byte, error) {
	var bytes []byte
	if !env.Publisher.IsZero() {
		var err error
		if bytes, err = vom.Encode(env.Publisher); err != nil {
			return nil, verror.New(errCantVOMEncodePublisher, nil, err)
		}
	}
	return json.Marshal(jsonType{
		Title:             env.Title,
		Args:              env.Args,
		Binary:            env.Binary,
		Publisher:         base64.URLEncoding.EncodeToString(bytes),
		Env:               env.Env,
		Packages:          env.Packages,
		Restarts:          env.Restarts,
		RestartTimeWindow: env.RestartTimeWindow,
	})
}

func (env *Envelope) UnmarshalJSON(input []byte) error {
	var jt jsonType
	if err := json.Unmarshal(input, &jt); err != nil {
		return err
	}
	var publisher security.Blessings
	if len(jt.Publisher) > 0 {
		bytes, err := base64.URLEncoding.DecodeString(jt.Publisher)
		if err != nil {
			return verror.New(errCantBase64DecodePublisher, nil, err)
		}
		if err := vom.Decode(bytes, &publisher); err != nil {
			return verror.New(errCantVOMDecodePublisher, nil, err)
		}
	}
	env.Title = jt.Title
	env.Args = jt.Args
	env.Binary = jt.Binary
	env.Publisher = publisher
	env.Env = jt.Env
	env.Packages = jt.Packages
	env.Restarts = jt.Restarts
	env.RestartTimeWindow = jt.RestartTimeWindow
	return nil
}
