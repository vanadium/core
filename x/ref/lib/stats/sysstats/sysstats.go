// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package sysstats implements system statistics and updates them periodically.
//
// Importing this package causes the stats to be exported via an init function.
package sysstats

import (
	"fmt"
	"os"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/mem"

	"v.io/v23/naming"
	"v.io/x/lib/metadata"
	"v.io/x/ref"
	"v.io/x/ref/lib/stats"
	"v.io/x/ref/lib/stats/counter"
)

var (
	counters     = map[string]*counter.Counter{}
	countersLock sync.Mutex
)

func init() {
	now := time.Now()
	stats.NewInteger("system/start-time-unix").Set(now.Unix())
	stats.NewString("system/start-time-rfc1123").Set(now.Format(time.RFC1123))
	stats.NewString("system/cmdline").Set(strings.Join(os.Args, " "))
	stats.NewInteger("system/num-cpu").Set(int64(runtime.NumCPU()))
	stats.NewIntegerFunc("system/num-goroutine", func() int64 { return int64(runtime.NumGoroutine()) })
	stats.NewString("system/version").Set(runtime.Version())
	stats.NewInteger("system/pid").Set(int64(os.Getpid()))
	if hostname, err := os.Hostname(); err == nil {
		stats.NewString("system/hostname").Set(hostname)
	}
	stats.NewIntegerFunc("system/GOMAXPROCS", func() int64 { return int64(runtime.GOMAXPROCS(0)) })
	exportEnv()
	exportMemStats()
	exportMetaData()
	exportSysMem()
	exportSysCPU()
	exportSysDisk()
}

func exportEnv() {
	var kv []stats.KeyValue
	for _, v := range os.Environ() {
		if parts := strings.SplitN(v, "=", 2); len(parts) == 2 {
			kv = append(kv, stats.KeyValue{
				Key:   parts[0],
				Value: parts[1],
			})
		}
	}
	stats.NewMap("system/environ").Set(kv)
}

func exportMemStats() {
	// Get field names to export.
	var memstats runtime.MemStats
	fieldNames := getFieldNames(memstats)
	updateStats := func() {
		var memstats runtime.MemStats
		runtime.ReadMemStats(&memstats)
		updateStats("system/memstats", memstats, fieldNames)
	}
	// Update stats now and every 10 seconds afterwards.
	updateStats()
	go func() {
		for {
			time.Sleep(10 * time.Second)
			updateStats()
		}
	}()
}

func exportMetaData() {
	var kv []stats.KeyValue
	for id, value := range metadata.ToMap() {
		kv = append(kv, stats.KeyValue{
			Key:   id,
			Value: value,
		})
	}
	stats.NewMap("system/metadata").Set(kv)
}

func exportSysMem() {
	// Get field names to export.
	var s mem.VirtualMemoryStat
	fieldNames := getFieldNames(s)

	// Update stats now and every 10 seconds afterwards.
	updateStats := func() {
		if s, err := mem.VirtualMemory(); err == nil {
			updateStats("system/sysmem", *s, fieldNames)
		}
	}
	go func() {
		for {
			updateStats()
			time.Sleep(10 * time.Second)
		}
	}()
}

func exportSysCPU() {
	// Get field names to export.
	type cpuStat struct {
		Percent   float64
		User      float64
		System    float64
		Idle      float64
		Nice      float64
		Iowait    float64
		Irq       float64
		Softirq   float64
		Steal     float64
		Guest     float64
		GuestNice float64
	}
	var s cpuStat
	fieldNames := getFieldNames(s)

	// Update stats now and every 10 seconds afterwards.
	updateStats := func() {
		pcts, err := cpu.Percent(time.Second*3, false)
		if err != nil || len(pcts) == 0 {
			return
		}
		times, err := cpu.Times(false)
		if err != nil || len(times) == 0 {
			return
		}
		t := times[0]
		s := cpuStat{
			Percent:   pcts[0],
			User:      t.User,
			System:    t.System,
			Idle:      t.Idle,
			Nice:      t.Nice,
			Iowait:    t.Iowait,
			Irq:       t.Irq,
			Softirq:   t.Softirq,
			Steal:     t.Steal,
			Guest:     t.Guest,
			GuestNice: t.GuestNice,
		}
		updateStats("system/syscpu", s, fieldNames)
	}
	go func() {
		for {
			updateStats()
			time.Sleep(10 * time.Second)
		}
	}()
}

func exportSysDisk() {
	strPaths := os.Getenv(ref.EnvSysStatsDiskPaths)
	if strPaths == "" {
		return
	}
	for _, path := range strings.Split(strPaths, ",") {
		// Get field names to export.
		var s disk.UsageStat
		fieldNames := getFieldNames(s)

		// Update stats now and every 10 seconds afterwards.
		curPath := path
		updateStats := func() {
			if s, err := disk.Usage(curPath); err == nil {
				updateStats(fmt.Sprintf("system/sysdisk/%s", naming.EncodeAsNameElement(curPath)), *s, fieldNames)
			}
		}
		go func() {
			for {
				updateStats()
				time.Sleep(10 * time.Second)
			}
		}()
	}
}

func getFieldNames(i interface{}) []string {
	fieldNames := []string{}
	v := reflect.ValueOf(i)
	v.FieldByNameFunc(func(name string) bool {
		switch v.FieldByName(name).Kind() {
		case reflect.Bool, reflect.Uint32, reflect.Uint64, reflect.Float64:
			fieldNames = append(fieldNames, name)
		}
		return false
	})
	return fieldNames
}

func updateStats(rootName string, s interface{}, fieldNames []string) { //nolint:gocyclo
	v := reflect.ValueOf(s)
	for _, name := range fieldNames {
		counterName := fmt.Sprintf("%s/%s", rootName, name)
		c := getCounter(counterName)
		i := v.FieldByName(name).Interface()
		v := int64(0)
		switch t := i.(type) {
		case bool:
			if t {
				v = 1
			}
		case int:
			v = int64(t)
		case int8:
			v = int64(t)
		case int16:
			v = int64(t)
		case int32:
			v = int64(t)
		case int64:
			v = t
		case uint:
			v = int64(t)
		case uint8:
			v = int64(t)
		case uint16:
			v = int64(t)
		case uint32:
			v = int64(t)
		case uint64:
			v = int64(t)
		case float32:
			v = int64(t)
		case float64:
			v = int64(t)
		default:
			fmt.Fprintf(os.Stderr, "not exporting %q (type %T not supported)", name, t)
			continue
		}
		c.Set(v)
	}
}

func getCounter(counterName string) *counter.Counter {
	countersLock.Lock()
	defer countersLock.Unlock()
	if _, ok := counters[counterName]; !ok {
		counters[counterName] = stats.NewCounter(counterName)
	}
	return counters[counterName]
}
