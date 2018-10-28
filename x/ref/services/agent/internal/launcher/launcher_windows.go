// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build windows

package launcher

import (
	"github.com/pkg/errors"
)

func LaunchAgent(credsDir, agentBin string, printCredsEnv bool, flags ...string) error {
	return errors.New("not supported")
}