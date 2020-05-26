// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package discovery

import (
	"bytes"
	"math/rand"
	"reflect"
	"testing"
	"testing/quick"
)

func TestUpdateBasic(t *testing.T) {
	rand := rand.New(rand.NewSource(0))
	for i := 0; i < 10; i++ {
		v, ok := quick.Value(reflect.TypeOf(AdInfo{}), rand)
		if !ok {
			t.Fatal("failed to populate advertisement information")
		}
		adinfo := v.Interface().(AdInfo)
		update := NewUpdate(&adinfo)

		if got, want := update.IsLost(), adinfo.Lost; got != want {
			t.Errorf("IsLost: got %v, but want %v", got, want)
		}
		if got, want := update.Id(), adinfo.Ad.Id; got != want {
			t.Errorf("Id: got %v, but want %v", got, want)
		}
		if got, want := update.Addresses(), adinfo.Ad.Addresses; !reflect.DeepEqual(got, want) {
			t.Errorf("Addresses: got %v, but want %v", got, want)
		}

		for k, v := range adinfo.Ad.Attributes {
			if got, want := update.Attribute(k), v; got != want {
				t.Errorf("Attribute[%q]: got %v, but want %v", k, got, want)
			}
		}

		for k, v := range adinfo.Ad.Attachments {
			r := <-update.Attachment(nil, k)
			if r.Error != nil {
				t.Errorf("Attachment[%q]: %v", k, r.Error)
				continue
			}
			if got, want := r.Data, v; !bytes.Equal(got, want) {
				t.Errorf("Attachment[%q]: got %v, but want %v", k, got, want)
			}

			// Make sure that attachments are copied.
			if len(v) > 0 {
				if got, want := &r.Data[0], &v[0]; got == want {
					t.Errorf("Attachment[%q]: not copied", k)
				}
			}
		}

		if got, want := update.Advertisement(), adinfo.Ad; !reflect.DeepEqual(got, want) {
			t.Errorf("Advertisement: got %v, but want %v", got, want)
		}
	}
}
