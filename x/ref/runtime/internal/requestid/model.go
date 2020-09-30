package requestid

import (
	"github.com/google/uuid"
	"v.io/v23/context"
)

type contextKey int

const requestIDKey = contextKey(iota)

func WithNewRequestID(ctx *context.T) *context.T {
	// Version 1 UUID based on the current NodeID and clock sequence, and the current time.
	requestID, err := uuid.NewUUID()
	if err != nil {
		panic(err)
	}
	return  context.WithValue(ctx, requestIDKey, requestID)
}

func RequestID(ctx *context.T) uuid.UUID {
	requestID := ctx.Value(requestIDKey).(uuid.UUID)
	return requestID
}
