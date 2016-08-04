This directory contains code uses to measure the performance of the Vanadium RPC stack.

---

This benchmarks use Go's testing package to run benchmarks. Each benchmark involves
one server and one client. The server has two very simple methods that echo the data
received from the client back to the client.

* `client ---- Echo(payload) ----> server`
* `client <--- return payload ---- server`

There are two versions of the Echo method:

* `Echo(payload []byte) ([]byte], error)`
* `EchoStream() <[]byte,[]byte> error`

# Microbenchmarks
## `benchmark_test.go`

The first benchmarks use the non-streaming version of Echo with a varying
payload size. The second benchmarks use the streaming version with varying
number of chunks and payload sizes. The third one is for measuring the
performance with multiple clients hosted in the same process.

This test creates a VC before the benchmark begins. So, the VC creation
overhead is excluded.

```
$ jiri go test -bench=. -timeout=1h -cpu=1 -benchtime=5s v.io/x/ref/runtime/internal/rpc/benchmark
PASS
Benchmark____1B     1000           8301357 ns/op           0.00 MB/s
--- Histogram (unit: ms)
        Count: 1000  Min: 7  Max: 17  Avg: 7.89
        ------------------------------------------------------------
        [  7,   8)   505   50.5%   50.5%  #####
        [  8,   9)   389   38.9%   89.4%  ####
        [  9,  10)    38    3.8%   93.2%
        [ 10,  11)    12    1.2%   94.4%
        [ 11,  12)     4    0.4%   94.8%
        [ 12,  14)    19    1.9%   96.7%
        [ 14,  16)    23    2.3%   99.0%
        [ 16,  18)    10    1.0%  100.0%
        [ 18,  21)     0    0.0%  100.0%
        [ 21,  24)     0    0.0%  100.0%
        [ 24, inf)     0    0.0%  100.0%
Benchmark___10B     1000           8587341 ns/op           0.00 MB/s
...
```

`RESULTS.txt` has the full benchmark results.

## `simple/main.go`

`simple/main.go` is a simple command-line tool to run the main benchmarks to measure
RPC setup time, latency, and throughput.

```
$ jiri go run simple/main.go
RPC Connection  33.48 ms/rpc
RPC (echo 1000B)  1.31 ms/rpc (763.05 qps)
RPC Streaming (echo 1000B)  0.11 ms/rpc
RPC Streaming Throughput (echo 1MB) 313.91 MB/s
```

# Client/Server
## `{benchmark,benchmarkd}/main.go`

`benchmarkd/main.go` and `benchmark/main.go` are simple command-line tools to run the
benchmark server and client as separate processes. Unlike the benchmarks above,
this test includes the startup cost of name resolution, creating the VC, etc. in
the first RPC.

```
$ jiri go run benchmarkd/main.go \
  -v23.tcp.address=localhost:8888 -v23.permissions.literal='{"Read": {"In": ["..."]}}'
```

(In a different shell)

```
$ jiri go run benchmark/main.go \
  -server=/localhost:8888 -iterations=100 -chunk_count=0 -payload_size=10
iterations: 100  chunk_count: 0  payload_size: 10
elapsed time: 1.369034277s
Histogram (unit: ms)
Count: 100  Min: 7  Max: 94  Avg: 13.17
------------------------------------------------------------
[  7,   8)    1    1.0%    1.0%
[  8,   9)    4    4.0%    5.0%
[  9,  10)   17   17.0%   22.0%  ##
[ 10,  12)   24   24.0%   46.0%  ##
[ 12,  15)   24   24.0%   70.0%  ##
[ 15,  19)   28   28.0%   98.0%  ###
[ 19,  24)    1    1.0%   99.0%
[ 24,  32)    0    0.0%   99.0%
[ 32,  42)    0    0.0%   99.0%
[ 42,  56)    0    0.0%   99.0%
[ 56,  75)    0    0.0%   99.0%
[ 75, 101)    1    1.0%  100.0%
[101, 136)    0    0.0%  100.0%
[136, 183)    0    0.0%  100.0%
[183, 247)    0    0.0%  100.0%
[247, 334)    0    0.0%  100.0%
[334, inf)    0    0.0%  100.0%
```

# Raspberry Pi

On a Raspberry Pi, everything is much slower. The same tests show the following
results:

```
$ ./main
RPC Connection  1765.47 ms/rpc
RPC (echo 1000B)  78.61 ms/rpc (12.72 qps)
RPC Streaming (echo 1000B)  23.85 ms/rpc
RPC Streaming Throughput (echo 1MB) 0.92 MB/s
```

On a Raspberry Pi 2,

```
$ ./main
RPC Connection  847.41 ms/rpc
RPC (echo 1000B)  16.47 ms/rpc (60.71 qps)
RPC Streaming (echo 1000B)  3.33 ms/rpc
RPC Streaming Throughput (echo 1MB) 2.31 MB/s
```
