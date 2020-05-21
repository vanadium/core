package context_test

import (
	gocontext "context"
	"testing"
	"time"

	vcontext "v.io/v23/context"
)

func TestNoopConversion(t *testing.T) {
	c0, cancel0 := vcontext.RootContext()
	c1 := vcontext.FromGoContext(c0)
	if c0 != c1 {
		t.Error("convert")
	}
	cancel0()
}

func TestFromGoContext(t *testing.T) {
	goctx, cancel := gocontext.WithCancel(gocontext.Background())
	c := vcontext.FromGoContext(goctx)
	if !c.Initialized() {
		t.Error("!initialized")
	}
	select {
	case <-c.Done():
		t.Error("done")
	default:
	}
	cancel()
	<-c.Done()
}

func TestDeadline(t *testing.T) {
	deadline := time.Now().Add(time.Second)
	goctx, cancel := gocontext.WithDeadline(gocontext.Background(), deadline)
	defer cancel()
	c := vcontext.FromGoContext(goctx)
	<-c.Done()
}

func TestValue(t *testing.T) {
	c0, cancel0 := vcontext.RootContext()
	type ts string // define a new type to avoid collisions with string
	c1 := vcontext.WithValue(c0, ts("foo1"), ts("bar1"))
	c2 := gocontext.WithValue(c1, ts("foo2"), ts("bar2"))
	c3 := vcontext.FromGoContext(c2)

	if v := c3.Value(ts("foo1")); v.(ts) != "bar1" {
		t.Error(v)
	}
	if v := c3.Value(ts("foo2")); v.(ts) != "bar2" {
		t.Error(v)
	}
	select {
	case <-c3.Done():
		t.Error("done")
	default:
	}
	cancel0()
	<-c1.Done()
	<-c2.Done()
	<-c3.Done()
}
