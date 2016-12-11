// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package servicetest

import (
	"fmt"
	"reflect"
	"sort"
	"testing"
)

func TestOneTape(t *testing.T) {
	tm := NewTapeMap()
	tm.ForSuffix("mytape").SetResponses("b", "c")
	if want, got := "b", tm.ForSuffix("mytape").Record("bar"); want != got {
		t.Errorf("Expected %v, got %v", want, got)
	}
	if want, got := "c", tm.ForSuffix("mytape").Record("baz"); want != got {
		t.Errorf("Expected %v, got %v", want, got)
	}
	if want, got := []interface{}{"bar", "baz"}, tm.ForSuffix("mytape").Play(); !reflect.DeepEqual(want, got) {
		t.Errorf("Expected %v, got %v", want, got)
	}
	tm.ForSuffix("mytape").Rewind()
	if want, got := []interface{}{}, tm.ForSuffix("mytape").Play(); !reflect.DeepEqual(want, got) {
		t.Errorf("Expected %v, got %v", want, got)
	}
}

func TestManyTapes(t *testing.T) {
	tm := NewTapeMap()
	tapes := []string{"duct tape", "cassette tape", "watergate tape", "tape worm"}
	for _, tp := range tapes {
		tm.ForSuffix(tp).SetResponses(tp + "resp")
	}
	for _, tp := range tapes {
		if want, got := tp+"resp", tm.ForSuffix(tp).Record(tp+"stimulus"); want != got {
			t.Errorf("Expected %v, got %v", want, got)
		}
	}
	for _, tp := range tapes {
		if want, got := []interface{}{tp + "stimulus"}, tm.ForSuffix(tp).Play(); !reflect.DeepEqual(want, got) {
			t.Errorf("Expected %v, got %v", want, got)
		}
	}
}

func TestTapeParallelism(t *testing.T) {
	tm := NewTapeMap()
	var resp []interface{}
	const N = 100
	for i := 0; i < N; i++ {
		resp = append(resp, fmt.Sprintf("resp%010d", i))
	}
	tm.ForSuffix("mytape").SetResponses(resp...)
	results := make(chan string, N)
	for i := 0; i < N; i++ {
		go func(index int) {
			results <- tm.ForSuffix("mytape").Record(fmt.Sprintf("stimulus%010d", index)).(string)
		}(i)
	}
	var res []string
	for i := 0; i < N; i++ {
		r := <-results
		res = append(res, r)
	}
	sort.Strings(res)
	var expectedRes []string
	for i := 0; i < N; i++ {
		expectedRes = append(expectedRes, fmt.Sprintf("resp%010d", i))
	}
	if want, got := expectedRes, res; !reflect.DeepEqual(want, got) {
		t.Errorf("Expected %v, got %v", want, got)
	}
	var expectedStimuli []string
	for i := 0; i < N; i++ {
		expectedStimuli = append(expectedStimuli, fmt.Sprintf("stimulus%010d", i))
	}
	var gotStimuli []string
	for _, s := range tm.ForSuffix("mytape").Play() {
		gotStimuli = append(gotStimuli, s.(string))
	}
	sort.Strings(gotStimuli)
	if want, got := expectedStimuli, gotStimuli; !reflect.DeepEqual(want, got) {
		t.Errorf("Expected %v, got %v", want, got)
	}
}
