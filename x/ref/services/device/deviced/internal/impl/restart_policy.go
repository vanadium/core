// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package impl

import (
	"time"

	"v.io/v23/services/application"
)

// RestartPolicy instances provide a policy for deciding if an
// application should be restarted on failure.
type restartPolicy interface {
	// decide determines if this application instance should be (re)started, returning
	// true if the application should be be (re)started.
	decide(envelope *application.Envelope, instance *instanceInfo) bool
}

type basicDecisionPolicy struct {
}

func newBasicRestartPolicy() restartPolicy {
	return new(basicDecisionPolicy)
}

func (rp *basicDecisionPolicy) decide(envelope *application.Envelope, instance *instanceInfo) bool {
	if envelope.Restarts == 0 {
		return false
	}
	if envelope.Restarts < 0 {
		return true
	}

	if instance.Restarts == 0 {
		instance.RestartWindowBegan = time.Now()
	}

	endOfWindow := instance.RestartWindowBegan.Add(envelope.RestartTimeWindow)
	if time.Now().After(endOfWindow) {
		instance.Restarts = 1
		instance.RestartWindowBegan = time.Now()
		return true
	}

	if instance.Restarts < envelope.Restarts {
		instance.Restarts++
		instance.RestartWindowBegan = time.Now()
		return true
	}

	return false

}
