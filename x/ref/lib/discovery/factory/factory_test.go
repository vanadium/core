// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package factory_test

import (
	"errors"
	"testing"

	"v.io/v23/context"
	"v.io/v23/discovery"

	"v.io/x/ref/lib/discovery/factory"
)

type mock struct {
	newErr                error
	numNews, numShutdowns int
}

func (m *mock) New(*context.T) (discovery.T, error) {
	m.numNews++
	return nil, m.newErr
}

func (m *mock) Shutdown() {
	m.numShutdowns++
}

func TestFactoryBasic(t *testing.T) {
	m := &mock{}
	factory.InjectFactory(m)

	f, _ := factory.New(nil)

	for i := 0; i < 3; i++ {
		_, err := f.New(nil)
		if err != nil {
			t.Error(err)
		}

		if want := i + 1; m.numNews != want {
			t.Errorf("expected %d New calls, but got %d times", want, m.numNews)
		}
	}

	m.newErr = errors.New("new error")
	if _, err := f.New(nil); err != m.newErr {
		t.Errorf("expected an error %v, but got %v", m.newErr, err)
	}

	f.Shutdown()
	if m.numShutdowns != 1 {
		t.Errorf("expected one Shutdown call, but got %d times", m.numShutdowns)
	}
}

func TestFactoryShutdownBeforeNew(t *testing.T) {
	m := &mock{}
	factory.InjectFactory(m)

	f, _ := factory.New(nil)

	f.Shutdown()
	if _, err := f.New(nil); err == nil {
		t.Error("expected an error, but got none")
	}
	if m.numNews != 0 {
		t.Errorf("expected no New call, but got %d times", m.numNews)
	}
	if m.numShutdowns != 0 {
		t.Errorf("expected no Shutdown call, but got %d times", m.numShutdowns)
	}
}
