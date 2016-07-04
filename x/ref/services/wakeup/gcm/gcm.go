// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package gcm defines functions for waking up remote services
// using Google Cloud Messaging (GCM).
package gcm

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	xmpp "github.com/mattn/go-xmpp"

	"v.io/v23/context"
	"v.io/v23/logging"
)

const (
	xmppHost     = "gcm.googleapis.com:5235"
	xmppNumConns = 3
	maxDelay     = 1 * time.Second
)

var (
	gcmMsgFmt = `
<message id=''>
  <gcm xmlns="google:mobile:data">
  %s
  </gcm>
</message>
`
	errDrain = errors.New("Draining")
	errStop  = errors.New("Explicit stop")
)

// NewGCMWakeup returns a wakeup function that uses GCM messaging to wakeup
// remote services.
func NewGCMWakeup(xmppUser, xmppPwd string, logger logging.Logger) func(*context.T, string, string) error {
	w := &wakeup{
		xmppUser: xmppUser,
		xmppPwd:  xmppPwd,
		send:     make(chan interface{}, 100),
		recv:     make(chan interface{}, 100),
		pending:  newPendingMap(),
		logger:   logger,
	}
	go w.recvLoop()
	for i := 0; i < xmppNumConns; i++ {
		go w.openConn()
	}
	return w.wakeup
}

type wakeup struct {
	xmppUser, xmppPwd string
	send, recv        chan interface{}
	pending           *pendingMap
	logger            logging.Logger
}

func (w *wakeup) wakeup(ctx *context.T, token, service string) error {
	id := fmt.Sprintf("%d", time.Now().UnixNano())
	c := make(chan error)
	w.pending.insert(id, c)
	defer w.pending.delete(id)
	msg := dataMessage{
		To: token,
		ID: id,
		Data: map[string]string{
			"vanadium_wakeup": "true",
			"message_id":      id,
			"service_name":    service,
		},
	}
	select {
	case w.send <- msg:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case err := <-c:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (w *wakeup) recvLoop() {
	for msg := range w.recv {
		switch m := msg.(type) {
		case *upstreamMessage:
			reply, ok := w.pending.get(m.ID)
			if !ok {
				// ID not found, which likely means that the wakeup request has timed out.
				continue
			}
			if errStr, ok := m.Data["error"]; ok && len(errStr) != 0 {
				reply <- errors.New(errStr)
			} else {
				reply <- nil // success
			}
		case *ackMessage:
			// Do nothing, for now.
		case *errorMessage:
			if reply, ok := w.pending.get(m.ID); ok {
				reply <- fmt.Errorf("Couldn't wakeup remote server: ", reply)
			}
		default:
			panic(fmt.Sprintf("Invalid message %v of type %T", m, m))
		}
	}
}

func (w *wakeup) openConn() {
	w.logger.Infof("Opening a new XMPP connection")
	var delay time.Duration
	for {
		<-time.After(delay)
		client, err := newXMPPClient(w.xmppUser, w.xmppPwd)
		if err != nil {
			// Retry, with exponential backoff (up to a maxDelay maximum).
			delay = backoff(delay)
			w.logger.Errorf("Couldn't open a new XMPP connection.  Retrying in %v: %v", delay, err)
			continue
		}
		go w.loop(client)
		return
	}
}

func (w *wakeup) loop(client *xmpp.Client) {
	drain := make(chan bool, 1)
	readDone := make(chan error, 1)
	writeDone := make(chan error, 1)
	writeStop := make(chan bool, 1)
	go w.readLoop(client, drain, readDone)
	go w.writeLoop(client, writeStop, drain, writeDone)
	select {
	case err := <-writeDone:
		w.logger.Errorf("XMPP write loop stopped with error: %v", err)
		w.openConn()
		if err != errDrain { // reader should stay alive while draining
			client.Close()
		}
		err = <-readDone
		w.logger.Errorf("XMPP read loop stopped with error: %v", err)
		client.Close()
	case err := <-readDone:
		w.logger.Errorf("XMPP read loop stopped with error: %v", err)
		w.openConn()
		writeStop <- true
		client.Close()
		err = <-writeDone
		w.logger.Errorf("XMPP write loop stopped with error: %v", err)
	}
}

func (w *wakeup) readLoop(client *xmpp.Client, drain chan<- bool, done chan<- error) {
	var draining bool
	for {
		m, err := client.Recv()
		if err != nil {
			done <- err
			return
		}
		msg, ok := m.(xmpp.Chat)
		if !ok {
			w.logger.Errorf("Unrecognized XMPP message: %v", m)
			continue
		}
		if control, ok := isControlMessage(msg); ok {
			if control.ControlType == "CONNECTION_DRAINING" {
				if !draining {
					drain <- true
					draining = true
				}
			} else {
				w.logger.Errorf("Unrecognized control message: %v", control)
			}
		} else if errMsg, ok := isErrorMessage(msg); ok {
			w.recv <- errMsg
		} else if ack, ok := isAckMessage(msg); ok {
			w.recv <- ack
		} else if up, ok := isUpstreamMessage(msg); ok {
			w.recv <- up
			// Send ACK.  This needs to be done on the same XMPP client, hence
			// the inline send here (as opposed to w.send <- ack).
			if err := sendAck(client, up.ID); err != nil {
				w.logger.Errorf("Couldn't ack:", err)
			}
		} else {
			w.logger.Errorf("Unrecognized XMPP chat message:", msg)
		}
	}
}

func (w *wakeup) writeLoop(client *xmpp.Client, stop <-chan bool, drain <-chan bool, done chan<- error) {
	for {
		select {
		case <-drain: // sent by the reader to signal that the connection is being drained
			done <- errDrain
			return
		case <-stop: // sent by the looper to signal that the writer should stop
			done <- errStop
			return
		case msg := <-w.send:
			jsonMsg, err := json.Marshal(msg)
			if err != nil {
				w.logger.Errorf("Invalid JSON message %v: %v", msg, err)
				continue
			}
			if err := sendMsg(client, string(jsonMsg)); err != nil {
				w.send <- msg
				done <- err
				return
			}
		}
	}
}

func newXMPPClient(user, pwd string) (c *xmpp.Client, err error) {
	return xmpp.NewClient(xmppHost, user, pwd, false)
}

func sendAck(c *xmpp.Client, id string) error {
	msg := ackMessage{
		Type: "ack",
		ID:   id,
	}
	jsonMsg, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return sendMsg(c, string(jsonMsg))
}

func sendMsg(c *xmpp.Client, jsonMsg string) error {
	gcmMsg := fmt.Sprintf(gcmMsgFmt, jsonMsg)
	n, err := c.SendOrg(gcmMsg)
	if err != nil {
		return err
	}
	if n != len(gcmMsg) {
		return fmt.Errorf("couldn't send all message bytes: want %d, sent %d", len(gcmMsg), n)
	}
	return nil
}

func isErrorMessage(msg xmpp.Chat) (*errorMessage, bool) {
	var errMsg errorMessage
	if len(msg.Other) < 1 {
		return nil, false
	}
	if err := json.Unmarshal([]byte(msg.Other[0]), &errMsg); err != nil {
		return nil, false
	}
	if msg.Type != "error" {
		return nil, false
	}
	return &errMsg, true
}

func isAckMessage(msg xmpp.Chat) (*ackMessage, bool) {
	var ack ackMessage
	if len(msg.Other) < 1 {
		return nil, false
	}
	if err := json.Unmarshal([]byte(msg.Other[0]), &ack); err != nil {
		return nil, false
	}
	if ack.Type != "ack" && ack.Type != "nack" {
		return nil, false
	}
	return &ack, true
}

func isUpstreamMessage(msg xmpp.Chat) (*upstreamMessage, bool) {
	var up upstreamMessage
	if len(msg.Other) < 1 {
		return nil, false
	}
	if err := json.Unmarshal([]byte(msg.Other[0]), &up); err != nil {
		return nil, false
	}
	if up.Category == "" {
		return nil, false
	}
	return &up, true
}

func isControlMessage(msg xmpp.Chat) (*controlMessage, bool) {
	var control controlMessage
	if len(msg.Other) < 1 {
		return nil, false
	}
	if err := json.Unmarshal([]byte(msg.Other[0]), &control); err != nil {
		return nil, false
	}
	if control.Type != "control" {
		return nil, false
	}
	return &control, true
}

type ackMessage struct {
	Type string `json:"message_type"`
	From string `json:"from"`
	ID   string `json:"message_id"`
}

type upstreamMessage struct {
	dataMessage
}

type errorMessage struct {
	dataMessage
}

type controlMessage struct {
	Type        string `json:"message_type"`
	ControlType string `json:"control_type"`
}

type dataMessage struct {
	To       string            `json:"to"`
	From     string            `json:"from"`
	Data     map[string]string `json:"data"`
	TTL      int               `json:"time_to_live"`
	ID       string            `json:"message_id"`
	Category string            `json:"category"`
}

func backoff(delay time.Duration) time.Duration {
	delay = 20*time.Millisecond + 2*delay
	if delay > maxDelay {
		delay = maxDelay
	}
	return delay
}

func newPendingMap() *pendingMap {
	return &pendingMap{
		pending: make(map[string]chan error),
	}
}

type pendingMap struct {
	lock    sync.RWMutex
	pending map[string]chan error
}

func (m *pendingMap) insert(id string, c chan error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.pending[id] = c
}

func (m *pendingMap) get(id string) (c chan error, ok bool) {
	m.lock.RLock()
	defer m.lock.RUnlock()
	c, ok = m.pending[id]
	return
}

func (m *pendingMap) delete(id string) {
	m.lock.Lock()
	defer m.lock.Unlock()
	delete(m.pending, id)
}
