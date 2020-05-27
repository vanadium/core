// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package basics

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"
)

//go:noinline
func nothing() {}

//go:noinline
func nothingTwo(a, b, c, d string) {}

func BenchmarkFunc(b *testing.B) {
	for i := 0; i < b.N; i++ {
		nothing()
	}
}

func BenchmarkFuncParams(b *testing.B) {
	for i := 0; i < b.N; i++ {
		nothingTwo("a", "b", "c", "d")
	}
}

func benchmarkLookupIntString(b *testing.B, mapSize, stringSize int) {
	mapStrings := make([]string, mapSize)
	intToStringMap := make(map[int]string, mapSize)
	for i := 0; i < mapSize; i++ {
		s := fmt.Sprintf("%0[2]*[1]d", i, stringSize)
		mapStrings[i] = s
		intToStringMap[i] = s
	}
	b.ResetTimer()
	// The following code is the same as:
	// for i := 0; i < b.N; i++ {
	//   _ = intToStringMap[i%mapSize]
	//  }
	// but it avoids doing the modulus operator
	// on each iteration.  This actually has an impact
	// on the measured time for such a cheap operation.
	for i := 0; i < b.N; i += mapSize {
		left := b.N - i
		if left > mapSize {
			left = mapSize
		}
		for j := 0; j < left; j++ {
			_ = intToStringMap[j]
		}
	}
}

func BenchmarkLookupInIntStringMap1M(b *testing.B) {
	benchmarkLookupIntString(b, 1<<20, 64)
}
func BenchmarkLookupInIntStringMap1K(b *testing.B) {
	benchmarkLookupIntString(b, 1<<10, 64)
}
func BenchmarkLookupInIntStringMap16(b *testing.B) {
	benchmarkLookupIntString(b, 16, 64)
}

func benchmarkLookupStringInt(b *testing.B, mapSize, stringSize int) {
	mapStrings := make([]string, mapSize)
	stringToIntMap := make(map[string]int, mapSize)
	for i := 0; i < mapSize; i++ {
		s := fmt.Sprintf("%0[2]*[1]d", i, stringSize)
		mapStrings[i] = s
		stringToIntMap[s] = i
	}
	b.ResetTimer()
	// The following code is the same as:
	// for i := 0; i < b.N; i++ {
	//   _ = stringToIntMap[mapStrings[i%mapSize]]
	//  }
	// but it avoids doing the modulus operator
	// on each iteration.  This actually has an impact
	// on the measured time for such a cheap operation.
	// Sadly I can't think of a way to remove the array
	// index, which pollutes these results.
	for i := 0; i < b.N; i += mapSize {
		left := b.N - i
		if left > mapSize {
			left = mapSize
		}
		for j := 0; j < left; j++ {
			_ = stringToIntMap[mapStrings[j]]
		}
	}
}

func BenchmarkLookupInStringIntMap1M(b *testing.B) {
	benchmarkLookupStringInt(b, 1<<20, 64)
}
func BenchmarkLookupInStringIntMap1K(b *testing.B) {
	benchmarkLookupStringInt(b, 1<<10, 64)
}
func BenchmarkLookupInStringIntMap1K_1K(b *testing.B) {
	benchmarkLookupStringInt(b, 1<<10, 1<<10)
}
func BenchmarkLookupInStringIntMap1K_10K(b *testing.B) {
	benchmarkLookupStringInt(b, 1<<10, 10*1<<10)
}
func BenchmarkLookupInStringIntMap16(b *testing.B) {
	benchmarkLookupStringInt(b, 16, 64)
}

func BenchmarkNow(b *testing.B) {
	for i := 0; i < b.N; i++ {
		time.Now()
	}
}

func BenchmarkSprintf(b *testing.B) {
	for i := 0; i < b.N; i++ {
		fmt.Sprintf("%d %d %d", i, i, i) //nolint:govet
	}
}

func benchmarkCopy(b *testing.B, size int) {
	data := make([]byte, 1<<20)
	cpy := make([]byte, size)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		copy(cpy, data)
	}
}

func BenchmarkCopy_16b(b *testing.B) { benchmarkCopy(b, 1<<4) }
func BenchmarkCopy_1k(b *testing.B)  { benchmarkCopy(b, 1<<10) }
func BenchmarkCopy_64k(b *testing.B) { benchmarkCopy(b, 1<<16) }
func BenchmarkCopy_1M(b *testing.B)  { benchmarkCopy(b, 1<<20) }

func benchmarkAlloc(b *testing.B, size int) {
	for i := 0; i < b.N; i++ {
		_ = make([]byte, size)
	}
}

func BenchmarkAlloc_16b(b *testing.B) { benchmarkAlloc(b, 1<<4) }
func BenchmarkAlloc_1k(b *testing.B)  { benchmarkAlloc(b, 1<<10) }
func BenchmarkAlloc_64k(b *testing.B) { benchmarkAlloc(b, 1<<16) }
func BenchmarkAlloc_1M(b *testing.B)  { benchmarkAlloc(b, 1<<20) }

func BenchmarkMutexUncontended(b *testing.B) {
	var mu sync.Mutex
	for i := 0; i < b.N; i++ {
		mu.Lock()
		mu.Unlock() //nolint:staticcheck  //lint:ignore SA2001
	}
}

func BenchmarkCallAsync(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ch := make(chan struct{})
		go func() {
			close(ch)
		}()
		<-ch
	}
}

func benchmarkWaitgroup(b *testing.B, n int) {
	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		wg.Add(n)
		for i := 0; i < n; i++ {
			go func() {
				wg.Done()
			}()
		}
		wg.Wait()
	}
}

func BenchmarkWaitgroup1(b *testing.B)    { benchmarkWaitgroup(b, 5) }
func BenchmarkWaitgroup10(b *testing.B)   { benchmarkWaitgroup(b, 10) }
func BenchmarkWaitgroup100(b *testing.B)  { benchmarkWaitgroup(b, 100) }
func BenchmarkWaitgroup1000(b *testing.B) { benchmarkWaitgroup(b, 1000) }

func w(b *testing.B, n, skip int, all bool) {
	x(b, n, skip, all)
}
func x(b *testing.B, n, skip int, all bool) {
	y(b, n, skip, all)
}
func y(b *testing.B, n, skip int, all bool) {
	z(b, n, skip, all)
}
func z(b *testing.B, n, skip int, all bool) {
	pcs := make([]uintptr, n)
	if all {
		for i := 0; i < b.N; i++ {
			runtime.Callers(skip, pcs)
		}
	} else {
		for i := 0; i < b.N; i++ {
			runtime.Caller(skip)
		}
	}
}

func BenchmarkCaller0(b *testing.B) { w(b, 0, 0, false) }
func BenchmarkCaller1(b *testing.B) { w(b, 0, 1, false) }
func BenchmarkCaller2(b *testing.B) { w(b, 0, 2, false) }
func BenchmarkCaller3(b *testing.B) { w(b, 0, 3, false) }
func BenchmarkCaller4(b *testing.B) { w(b, 0, 4, false) }

func BenchmarkCallers0(b *testing.B) { w(b, 5, 0, true) }
func BenchmarkCallers1(b *testing.B) { w(b, 5, 1, true) }
func BenchmarkCallers2(b *testing.B) { w(b, 5, 2, true) }
func BenchmarkCallers3(b *testing.B) { w(b, 5, 3, true) }
func BenchmarkCallers4(b *testing.B) { w(b, 5, 4, true) }

func BenchmarkCallersFrames1(b *testing.B) { w(b, 1, 1, true) }
func BenchmarkCallersFrames2(b *testing.B) { w(b, 2, 1, true) }
func BenchmarkCallersFrames3(b *testing.B) { w(b, 3, 1, true) }
func BenchmarkCallersFrames4(b *testing.B) { w(b, 4, 1, true) }

func BenchmarkFuncForPC(b *testing.B) {
	callers := make([]uintptr, 1)
	runtime.Callers(1, callers)
	caller := callers[0]
	for i := 0; i < b.N; i++ {
		runtime.FuncForPC(caller).FileLine(caller)
	}
}

func benchmarkRoundtrip(b *testing.B, network, addr string) {
	l, err := net.Listen(network, addr)
	if err != nil {
		b.Fatal(err)
	}
	errCh := make(chan error, 1)
	var wg sync.WaitGroup
	defer func() {
		l.Close()
		wg.Wait()
		close(errCh)
		for err := range errCh {
			if err != nil {
				b.Fatal(err)
			}
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		c, err := l.Accept()
		if err != nil {
			errCh <- err
			return
		}
		defer c.Close()
		buf := make([]byte, 1)
		for i := 0; i < b.N; i++ {
			if _, err := c.Read(buf); err != nil {
				errCh <- err
				return
			}
			if _, err := c.Write(buf); err != nil {
				errCh <- err
				return
			}
		}
		errCh <- nil
	}()

	c, err := net.Dial(l.Addr().Network(), l.Addr().String())
	if err != nil {
		b.Fatal(err)
	}
	defer c.Close()
	buf := make([]byte, 1)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := c.Write(buf); err != nil {
			b.Fatal(err)
		}
		if _, err := c.Read(buf); err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()
}

func BenchmarkRoundtripTCP(b *testing.B) {
	benchmarkRoundtrip(b, "tcp", "127.0.0.1:0")
}

func BenchmarkRoundtripUnix(b *testing.B) {
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(dir)
	benchmarkRoundtrip(b, "unix", filepath.Join(dir, "echo.sock"))
}

func BenchmarkRoundtripTCP_C(b *testing.B) {
	if err := cRoundTrip(b); err != nil {
		b.Fatal(err)
	}
}
