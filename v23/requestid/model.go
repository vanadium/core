package requestid

import (
	"github.com/google/uuid"
	"v.io/v23/context"
)

type contextKey int

const requestIDKey = contextKey(iota)

func WithNewRequestID(ctx *context.T) *context.T {
	requestID := uuid.New()
	return  context.WithValue(ctx, requestIDKey, requestID)
}


func RequestID(ctx *context.T) uuid.UUID {
	requestID := ctx.Value(requestIDKey).(uuid.UUID)
	return requestID
}