# Package [cloudeng.io/os/lockedfile/internal/testenv](https://pkg.go.dev/cloudeng.io/os/lockedfile/internal/testenv?tab=doc)
[![CircleCI](https://circleci.com/gh/cloudengio/go.gotools.svg?style=svg)](https://circleci.com/gh/cloudengio/go.gotools) [![Go Report Card](https://goreportcard.com/badge/cloudeng.io/os/lockedfile/internal/testenv)](https://goreportcard.com/report/cloudeng.io/os/lockedfile/internal/testenv)

```go
import cloudeng.io/os/lockedfile/internal/testenv
```


## Functions
### Func HasExec
```go
func HasExec() bool
```
HasExec reports whether the current system can start new processes using
os.StartProcess or (more commonly) exec.Command.

### Func MustHaveExec
```go
func MustHaveExec(t testing.TB)
```
MustHaveExec checks that the current system can start new processes using
os.StartProcess or (more commonly) exec.Command. If not, MustHaveExec calls
t.Skip with an explanation.




