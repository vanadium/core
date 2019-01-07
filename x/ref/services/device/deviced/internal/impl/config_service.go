// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package impl

// The config invoker is responsible for answering calls to the config service
// run as part of the device manager.  The config invoker converts RPCs to
// messages on channels that are used to listen on callbacks coming from child
// application instances.

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/rpc"
	"v.io/v23/verror"
	"v.io/x/ref/services/device/internal/errors"
)

type callbackState struct {
	sync.Mutex
	// channels maps callback identifiers and config keys to channels that
	// are used to communicate corresponding config values from child
	// processes.
	channels map[string]map[string]chan<- string
	// nextCallbackID provides the next callback identifier to use as a key
	// for the channels map.
	nextCallbackID int64
	// name is the object name for making calls against the device manager's
	// config service.
	name string
}

func newCallbackState(name string) *callbackState {
	return &callbackState{
		channels: make(map[string]map[string]chan<- string),
		name:     name,
	}
}

// callbackListener abstracts out listening for values provided via the
// callback mechanism for a given key.
type callbackListener interface {
	// waitForValue blocks until the value that this listener is expecting
	// arrives, until the timeout expires, or until stop() is called
	waitForValue(timeout time.Duration) (string, error)
	// stop makes waitForValue return early
	stop()
	// cleanup cleans up any state used by the listener.  Should be called
	// when the listener is no longer needed.
	cleanup()
	// name returns the object name for the config service object that
	// handles the key that the listener is listening for.
	name() string
}

// listener implements callbackListener
type listener struct {
	ctx     *context.T
	id      string
	cs      *callbackState
	ch      <-chan string
	n       string
	stopper chan struct{}
}

func (l *listener) waitForValue(timeout time.Duration) (string, error) {
	select {
	case value := <-l.ch:
		return value, nil
	case <-time.After(timeout):
		return "", verror.New(errors.ErrOperationFailed, nil, fmt.Sprintf("Waiting for callback timed out after %v", timeout))
	case <-l.stopper:
		return "", verror.New(errors.ErrOperationFailed, nil, fmt.Sprintf("Stopped while waiting for callback"))
	}
}

func (l *listener) stop() {
	l.ctx.FlushLog()
	close(l.stopper)
}

func (l *listener) cleanup() {
	l.cs.unregister(l.id)
}

func (l *listener) name() string {
	return l.n
}

func (c *callbackState) listenFor(ctx *context.T, key string) callbackListener {
	id := c.generateID()
	callbackName := naming.Join(c.name, configSuffix, id)
	// Make the channel buffered to avoid blocking the Set method when
	// nothing is receiving on the channel.  This happens e.g. when
	// unregisterCallbacks executes before Set is called.
	callbackChan := make(chan string, 1)
	c.register(id, key, callbackChan)
	stopchan := make(chan struct{}, 1)
	return &listener{
		ctx:     ctx,
		id:      id,
		cs:      c,
		ch:      callbackChan,
		n:       callbackName,
		stopper: stopchan,
	}
}

func (c *callbackState) generateID() string {
	c.Lock()
	defer c.Unlock()
	c.nextCallbackID++
	return strconv.FormatInt(c.nextCallbackID-1, 10)
}

func (c *callbackState) register(id, key string, channel chan<- string) {
	c.Lock()
	defer c.Unlock()
	if _, ok := c.channels[id]; !ok {
		c.channels[id] = make(map[string]chan<- string)
	}
	c.channels[id][key] = channel
}

func (c *callbackState) unregister(id string) {
	c.Lock()
	defer c.Unlock()
	delete(c.channels, id)
}

// configService implements the Device manager's Config interface.
type configService struct {
	callback *callbackState
	// Suffix contains an identifier for the channel corresponding to the
	// request.
	suffix string
}

func (i *configService) Set(_ *context.T, _ rpc.ServerCall, key, value string) error {
	id := i.suffix
	i.callback.Lock()
	if _, ok := i.callback.channels[id]; !ok {
		i.callback.Unlock()
		return verror.New(errors.ErrInvalidSuffix, nil)
	}
	channel, ok := i.callback.channels[id][key]
	i.callback.Unlock()
	if !ok {
		return nil
	}
	channel <- value
	return nil
}
