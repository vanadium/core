// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rpc

import (
	"github.com/google/uuid"

	"v.io/v23/context"
)

type contextKey int

const requestIDKey = contextKey(iota)

func WithRequestID(ctx *context.T, requestID uuid.UUID) *context.T {
	return context.WithValue(ctx, requestIDKey, requestID)
}

func RequestID(ctx *context.T) uuid.UUID {
	requestID, _ := ctx.Value(requestIDKey).(uuid.UUID)
	return requestID
}
