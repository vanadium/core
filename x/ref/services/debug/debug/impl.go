// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The following enables go generate to generate the doc.go file.
//go:generate go run v.io/x/lib/cmdline/gendoc .

package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/glob"
	"v.io/v23/naming"
	"v.io/v23/services/logreader"
	"v.io/v23/services/pprof"
	"v.io/v23/services/stats"
	s_vtrace "v.io/v23/services/vtrace"
	"v.io/v23/services/watch"
	"v.io/v23/uniqueid"
	"v.io/v23/vdl"
	"v.io/v23/vom"
	"v.io/v23/vtrace"
	"v.io/x/lib/cmdline"
	"v.io/x/ref/lib/signals"
	"v.io/x/ref/lib/v23cmd"
	_ "v.io/x/ref/runtime/factories/roaming"
	"v.io/x/ref/services/internal/pproflib"
)

func main() {
	cmdline.Main(cmdRoot)
}

var (
	follow     bool
	verbose    bool
	numEntries int
	startPos   int64
	raw        bool
	rawJSON    bool
	showType   bool
	pprofCmd   string
	timeout    time.Duration
)

func init() {
	cmdline.HideGlobalFlagsExcept()

	cmdRoot.Flags.DurationVar(&timeout, "timeout", 10*time.Second, "Time to wait for various RPCs")

	// logs read flags
	cmdLogsRead.Flags.BoolVar(&follow, "f", false, "When true, read will wait for new log entries when it reaches the end of the file.")
	cmdLogsRead.Flags.BoolVar(&verbose, "v", false, "When true, read will be more verbose.")
	cmdLogsRead.Flags.IntVar(&numEntries, "n", int(logreader.AllEntries), "The number of log entries to read.")
	cmdLogsRead.Flags.Int64Var(&startPos, "o", 0, "The position, in bytes, from which to start reading the log file.")

	// stats read flags
	cmdStatsRead.Flags.BoolVar(&raw, "raw", false, "When true, the command will display the raw value of the object.")
	cmdStatsRead.Flags.BoolVar(&rawJSON, "json", false, "When true, the command will display the raw value of the object in json format.")
	cmdStatsRead.Flags.BoolVar(&showType, "type", false, "When true, the type of the values will be displayed.")

	// stats watch flags
	cmdStatsWatch.Flags.BoolVar(&raw, "raw", false, "When true, the command will display the raw value of the object.")
	cmdStatsWatch.Flags.BoolVar(&showType, "type", false, "When true, the type of the values will be displayed.")

	// pprof flags
	cmdPProfRun.Flags.StringVar(&pprofCmd, "pprofcmd", "jiri go tool pprof", "The pprof command to use.")
}

var cmdVtrace = &cmdline.Command{
	Runner:   v23cmd.RunnerFunc(runVtrace),
	Name:     "vtrace",
	Short:    "Returns vtrace traces.",
	Long:     "Returns matching vtrace traces (or all stored traces if no ids are given).",
	ArgsName: "<name> [id ...]",
	ArgsLong: `
<name> is the name of a vtrace object.
[id] is a vtrace trace id.
`,
}

func doFetchTrace(ctx *context.T, wg *sync.WaitGroup, client s_vtrace.StoreClientStub,
	id uniqueid.Id, traces chan *vtrace.TraceRecord, errors chan error) {
	defer wg.Done()

	trace, err := client.Trace(ctx, id)
	if err != nil {
		errors <- err
	} else {
		traces <- &trace
	}
}

func runVtrace(ctx *context.T, env *cmdline.Env, args []string) error {
	arglen := len(args)
	if arglen == 0 {
		return env.UsageErrorf("vtrace: incorrect number of arguments, got %d want >= 1", arglen)
	}

	name := args[0]
	client := s_vtrace.StoreClient(name)
	if arglen == 1 {
		call, err := client.AllTraces(ctx)
		if err != nil {
			return err
		}
		stream := call.RecvStream()
		for stream.Advance() {
			trace := stream.Value()
			vtrace.FormatTrace(os.Stdout, &trace, nil)
		}
		if err := stream.Err(); err != nil {
			return err
		}
		return call.Finish()
	}

	ntraces := len(args) - 1
	traces := make(chan *vtrace.TraceRecord, ntraces)
	errors := make(chan error, ntraces)
	var wg sync.WaitGroup
	wg.Add(ntraces)
	for _, arg := range args[1:] {
		id, err := uniqueid.FromHexString(arg)
		if err != nil {
			return err
		}
		go doFetchTrace(ctx, &wg, client, id, traces, errors)
	}
	go func() {
		wg.Wait()
		close(traces)
		close(errors)
	}()

	for trace := range traces {
		vtrace.FormatTrace(os.Stdout, trace, nil)
	}

	// Just return one of the errors.
	return <-errors
}

var cmdGlob = &cmdline.Command{
	Runner:   v23cmd.RunnerFunc(runGlob),
	Name:     "glob",
	Short:    "Returns all matching entries from the namespace.",
	Long:     "Returns all matching entries from the namespace.",
	ArgsName: "<pattern> ...",
	ArgsLong: `
<pattern> is a glob pattern to match.
`,
}

func runGlob(ctx *context.T, env *cmdline.Env, args []string) error {
	if min, got := 1, len(args); got < min {
		return env.UsageErrorf("glob: incorrect number of arguments, got %d, want >=%d", got, min)
	}
	results := make(chan naming.GlobReply)
	errors := make(chan error)
	doGlobs(ctx, args, results, errors)
	var lastErr error
	for {
		select {
		case err := <-errors:
			lastErr = err
			fmt.Fprintln(env.Stderr, "Error:", err)
		case me, ok := <-results:
			if !ok {
				return lastErr
			}
			switch v := me.(type) {
			case *naming.GlobReplyEntry:
				fmt.Fprint(env.Stdout, v.Value.Name)
				for _, s := range v.Value.Servers {
					fmt.Fprintf(env.Stdout, " %s (Deadline %s)", s.Server, s.Deadline.Time)
				}
				fmt.Fprintln(env.Stdout)
			case *naming.GlobReplyError:
				fmt.Fprintf(env.Stderr, "Error: %s: %v\n", v.Value.Name, v.Value.Error)
			}
		}
	}
}

// doGlobs calls Glob on multiple patterns in parallel and sends all the results
// on the results channel and all the errors on the errors channel. It closes
// the results channel when all the results have been sent.
func doGlobs(ctx *context.T, patterns []string, results chan<- naming.GlobReply, errors chan<- error) {
	var wg sync.WaitGroup
	wg.Add(len(patterns))
	for _, p := range patterns {
		go doGlob(ctx, p, results, errors, &wg)
	}
	go func() {
		wg.Wait()
		close(results)
	}()
}

func doGlob(ctx *context.T, pattern string, results chan<- naming.GlobReply, errors chan<- error, wg *sync.WaitGroup) {
	defer wg.Done()
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	c, err := v23.GetNamespace(ctx).Glob(ctx, pattern)
	if err != nil {
		errors <- fmt.Errorf("%s: %v", pattern, err)
		return
	}
	for me := range c {
		results <- me
	}
}

var cmdLogsRead = &cmdline.Command{
	Runner:   v23cmd.RunnerFunc(runLogsRead),
	Name:     "read",
	Short:    "Reads the content of a log file object.",
	Long:     "Reads the content of a log file object.",
	ArgsName: "<name>",
	ArgsLong: `
<name> is the name of the log file object.
`,
}

func runLogsRead(ctx *context.T, env *cmdline.Env, args []string) error {
	if want, got := 1, len(args); want != got {
		return env.UsageErrorf("read: incorrect number of arguments, got %d, want %d", got, want)
	}
	name := args[0]
	if !follow {
		var cancel func()
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	lf := logreader.LogFileClient(name)
	stream, err := lf.ReadLog(ctx, startPos, int32(numEntries), follow)
	if err != nil {
		return err
	}
	iterator := stream.RecvStream()
	for iterator.Advance() {
		entry := iterator.Value()
		if verbose {
			fmt.Fprintf(env.Stdout, "[%d] %s\n", entry.Position, entry.Line)
		} else {
			fmt.Fprintf(env.Stdout, "%s\n", entry.Line)
		}
	}
	if err = iterator.Err(); err != nil {
		return err
	}
	offset, err := stream.Finish()
	if err != nil {
		return err
	}
	if verbose {
		fmt.Fprintf(env.Stdout, "Offset: %d\n", offset)
	}
	return nil
}

var cmdLogsSize = &cmdline.Command{
	Runner:   v23cmd.RunnerFunc(runLogsSize),
	Name:     "size",
	Short:    "Returns the size of a log file object.",
	Long:     "Returns the size of a log file object.",
	ArgsName: "<name>",
	ArgsLong: `
<name> is the name of the log file object.
`,
}

func runLogsSize(ctx *context.T, env *cmdline.Env, args []string) error {
	if want, got := 1, len(args); want != got {
		return env.UsageErrorf("size: incorrect number of arguments, got %d, want %d", got, want)
	}
	name := args[0]
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	lf := logreader.LogFileClient(name)
	size, err := lf.Size(ctx)
	if err != nil {
		return err
	}
	fmt.Fprintln(env.Stdout, size)
	return nil
}

var cmdStatsRead = &cmdline.Command{
	Runner:   v23cmd.RunnerFunc(runStatsRead),
	Name:     "read",
	Short:    "Returns the value of stats objects.",
	Long:     "Returns the value of stats objects.",
	ArgsName: "<name> ...",
	ArgsLong: `
<name> is the name of a stats object, or a glob pattern to match against stats
object names.
`,
}

func runStatsRead(ctx *context.T, env *cmdline.Env, args []string) error {
	if min, got := 1, len(args); got < min {
		return env.UsageErrorf("read: incorrect number of arguments, got %d, want >=%d", got, min)
	}
	globResults := make(chan naming.GlobReply)
	errors := make(chan error)
	doGlobs(ctx, args, globResults, errors)

	output := make(chan string)
	go func() {
		var wg sync.WaitGroup
		for me := range globResults {
			if v, ok := me.(*naming.GlobReplyEntry); ok {
				wg.Add(1)
				go doValue(ctx, v.Value.Name, output, errors, &wg)
			}
		}
		wg.Wait()
		close(output)
	}()

	var lastErr error
	jsonOutputs := []string{}
	for {
		select {
		case err := <-errors:
			lastErr = err
			fmt.Fprintln(env.Stderr, err)
		case out, ok := <-output:
			if !ok {
				if rawJSON {
					fmt.Fprintf(env.Stdout, "[%s]", strings.Join(jsonOutputs, ","))
				}
				return lastErr
			}
			if rawJSON {
				jsonOutputs = append(jsonOutputs, out)
			} else {
				fmt.Fprintln(env.Stdout, out)
			}
		}
	}
}

func doValue(ctx *context.T, name string, output chan<- string, errors chan<- error, wg *sync.WaitGroup) {
	defer wg.Done()
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	v, err := stats.StatsClient(name).Value(ctx)
	if err != nil {
		errors <- fmt.Errorf("%s: %v", name, err)
		return
	}
	fv, err := formatValue(v)
	if err != nil {
		errors <- fmt.Errorf("%s: %v", name, err)
		// fv is still valid, so dump it out too.
	}
	// Add "name" to the returned json string if "-json" flag is set.
	if rawJSON {
		output <- strings.Replace(fv, "{", fmt.Sprintf(`{"Name":%q,`, name), 1)
	} else {
		output <- fmt.Sprintf("%s: %v", name, fv)
	}
}

var cmdStatsWatch = &cmdline.Command{
	Runner:   v23cmd.RunnerFunc(runStatsWatch),
	Name:     "watch",
	Short:    "Returns a stream of all matching entries and their values as they change.",
	Long:     "Returns a stream of all matching entries and their values as they change.",
	ArgsName: "<pattern> ...",
	ArgsLong: `
<pattern> is a glob pattern to match.
`,
}

func runStatsWatch(ctx *context.T, env *cmdline.Env, args []string) error {
	if want, got := 1, len(args); got < want {
		return env.UsageErrorf("watch: incorrect number of arguments, got %d, want >=%d", got, want)
	}

	results := make(chan string)
	errors := make(chan error)
	var wg sync.WaitGroup
	wg.Add(len(args))
	for _, arg := range args {
		go doWatch(ctx, arg, results, errors, &wg)
	}
	go func() {
		wg.Wait()
		close(results)
	}()
	var lastErr error
	for {
		select {
		case err := <-errors:
			lastErr = err
			fmt.Fprintln(env.Stderr, "Error:", err)
		case r, ok := <-results:
			if !ok {
				return lastErr
			}
			fmt.Fprintln(env.Stdout, r)
		}
	}
}

func doWatch(ctx *context.T, pattern string, results chan<- string, errors chan<- error, wg *sync.WaitGroup) {
	defer wg.Done()
	root, globPattern := naming.SplitAddressName(pattern)
	g, err := glob.Parse(globPattern)
	if err != nil {
		errors <- fmt.Errorf("%s: %v", globPattern, err)
		return
	}
	var prefixElems []string
	prefixElems, g = g.SplitFixedElements()
	name := naming.Join(prefixElems...)
	if len(root) != 0 {
		name = naming.JoinAddressName(root, name)
	}
	c := watch.GlobWatcherClient(name)
	for retry := false; ; retry = true {
		if retry {
			time.Sleep(10 * time.Second)
		}
		stream, err := c.WatchGlob(ctx, watch.GlobRequest{Pattern: g.String()})
		if err != nil {
			errors <- fmt.Errorf("%s: %v", name, err)
			continue
		}
		iterator := stream.RecvStream()
		for iterator.Advance() {
			v := iterator.Value()
			fv, err := formatValue(v.Value)
			if err != nil {
				errors <- fmt.Errorf("%s: %v", name, err)
				// fv is still valid, so dump it out too.
			}
			results <- fmt.Sprintf("%s: %s", naming.Join(name, v.Name), fv)
		}
		if err = iterator.Err(); err != nil {
			errors <- fmt.Errorf("%s: %v", name, err)
			continue
		}
		if err = stream.Finish(); err != nil {
			errors <- fmt.Errorf("%s: %v", name, err)
		}
	}
}

func formatValue(rb *vom.RawBytes) (string, error) {
	value := vdl.ValueOf(rb)
	var ret string
	if showType {
		ret += value.Type().String() + ": "
	}
	if rawJSON {
		var converted interface{}
		// We will just return raw string if any of the following steps fails.
		if err := vdl.Convert(&converted, value); err != nil {
			retErr := fmt.Errorf("couldn't show raw content in json format: %v", err)
			return ret + value.String(), retErr
		}
		result := struct {
			Type  string
			Value interface{}
		}{
			Value: converted,
		}
		if showType {
			result.Type = value.Type().String()
		}
		resultBytes, err := json.Marshal(result)
		if err != nil {
			retErr := fmt.Errorf("couldn't show raw content in json format: %v", err)
			return ret + value.String(), retErr
		}
		return string(resultBytes), nil
	}
	if raw {
		return ret + value.String(), nil
	}
	// Convert the *vdl.Value into an interface{}, so that things like histograms
	// get pretty-printed.
	var pretty interface{}
	err := vdl.Convert(&pretty, value)
	if err != nil {
		// If we can't convert, just print the raw value, but still return an error.
		err = fmt.Errorf("couldn't pretty-print, consider setting -raw: %v", err)
		pretty = value
	}
	return ret + fmt.Sprint(pretty), err
}

var cmdPProfRun = &cmdline.Command{
	Runner:   v23cmd.RunnerFunc(runPProf),
	Name:     "run",
	Short:    "Runs the pprof tool.",
	Long:     "Runs the pprof tool.",
	ArgsName: "<name> <profile> [passthru args] ...",
	ArgsLong: `
<name> is the name of the pprof object.
<profile> the name of the profile to use.

All the [passthru args] are passed to the pprof tool directly, e.g.

  $ debug pprof run a/b/c/__debug/pprof heap --text
  $ debug pprof run a/b/c/__debug/pprof profile -gv
`,
}

func runPProf(ctx *context.T, env *cmdline.Env, args []string) error {
	if min, got := 1, len(args); got < min {
		return env.UsageErrorf("pprof: incorrect number of arguments, got %d, want >=%d", got, min)
	}
	name := args[0]
	if len(args) == 1 {
		return showPProfProfiles(ctx, env, name)
	}
	profile := args[1]
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	addr, err := startPprofProxyHTTPServer(ctx, name)
	if err != nil {
		return err
	}
	// Construct the pprof command line:
	// <pprofCmd> http://<proxyaddr>/pprof/<profile> [pprof flags]
	pargs := []string{pprofCmd} // pprofCmd is purposely not escaped.
	for i := 2; i < len(args); i++ {
		pargs = append(pargs, shellEscape(args[i]))
	}
	pargs = append(pargs, shellEscape(fmt.Sprintf("http://%s/pprof/%s", addr, profile)))
	pcmd := strings.Join(pargs, " ")
	fmt.Fprintf(env.Stdout, "Running: %s\n", pcmd)
	c := exec.Command("sh", "-c", pcmd)
	c.Stdin = os.Stdin
	c.Stdout = env.Stdout
	c.Stderr = env.Stderr
	return c.Run()
}

func showPProfProfiles(ctx *context.T, env *cmdline.Env, name string) error {
	v, err := pprof.PProfClient(name).Profiles(ctx)
	if err != nil {
		return err
	}
	v = append(v, "profile")
	sort.Strings(v)
	fmt.Fprintln(env.Stdout, "Available profiles:")
	for _, p := range v {
		fmt.Fprintf(env.Stdout, "  %s\n", p)
	}
	return nil
}

func shellEscape(s string) string {
	if !strings.Contains(s, "'") {
		return "'" + s + "'"
	}
	re := regexp.MustCompile("([\"$`\\\\])")
	return `"` + re.ReplaceAllString(s, "\\$1") + `"`
}

var cmdPProfRunProxy = &cmdline.Command{
	Runner:   v23cmd.RunnerFunc(runPProfProxy),
	Name:     "proxy",
	Short:    "Runs an http proxy to a pprof object.",
	Long:     "Runs an http proxy to a pprof object.",
	ArgsName: "<name>",
	ArgsLong: `
<name> is the name of the pprof object.
`,
}

func runPProfProxy(ctx *context.T, env *cmdline.Env, args []string) error {
	if want, got := 1, len(args); got != want {
		return env.UsageErrorf("proxy: incorrect number of arguments, got %d, want %d", got, want)
	}
	addr, err := startPprofProxyHTTPServer(ctx, args[0])
	if err != nil {
		return err
	}
	fmt.Fprintln(env.Stdout)
	fmt.Fprintf(env.Stdout, "The pprof proxy is listening at http://%s/pprof\n", addr)
	fmt.Fprintln(env.Stdout)
	fmt.Fprintln(env.Stdout, "Hit CTRL-C to exit")

	<-signals.ShutdownOnSignals(ctx)
	return nil
}

var cmdRoot = &cmdline.Command{
	Name:  "debug",
	Short: "supports debugging Vanadium servers.",
	Long:  "Command debug supports debugging Vanadium servers.",
	Children: []*cmdline.Command{
		cmdGlob,
		cmdVtrace,
		{
			Name:     "logs",
			Short:    "Accesses log files",
			Long:     "Accesses log files",
			Children: []*cmdline.Command{cmdLogsRead, cmdLogsSize},
		},
		{
			Name:     "stats",
			Short:    "Accesses stats",
			Long:     "Accesses stats",
			Children: []*cmdline.Command{cmdStatsRead, cmdStatsWatch},
		},
		{
			Name:     "pprof",
			Short:    "Accesses profiling data",
			Long:     "Accesses profiling data",
			Children: []*cmdline.Command{cmdPProfRun, cmdPProfRunProxy},
		},
		cmdBrowse,
		cmdDelegate,
		cmdDiscovery,
	},
}

func startPprofProxyHTTPServer(ctx *context.T, name string) (string, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	http.Handle("/", pproflib.PprofProxy(ctx, "", name))
	go http.Serve(ln, nil) //nolint:errcheck
	go func() {
		<-ctx.Done()
		ln.Close()
	}()
	return ln.Addr().String(), nil
}
