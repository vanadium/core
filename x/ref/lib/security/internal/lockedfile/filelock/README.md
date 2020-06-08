# Package [cloudeng.io/os/lockedfile/internal/filelock](https://pkg.go.dev/cloudeng.io/os/lockedfile/internal/filelock?tab=doc)
[![CircleCI](https://circleci.com/gh/cloudengio/go.gotools.svg?style=svg)](https://circleci.com/gh/cloudengio/go.gotools) [![Go Report Card](https://goreportcard.com/badge/cloudeng.io/os/lockedfile/internal/filelock)](https://goreportcard.com/report/cloudeng.io/os/lockedfile/internal/filelock)

```go
import cloudeng.io/os/lockedfile/internal/filelock
```

Package filelock provides a platform-independent API for advisory file
locking. Calls to functions in this package on platforms that do not support
advisory locks will return errors for which IsNotSupported returns true.

## Variables
### ErrNotSupported
```go
ErrNotSupported = errors.New("operation not supported")

```



## Functions
### Func IsNotSupported
```go
func IsNotSupported(err error) bool
```
IsNotSupported returns a boolean indicating whether the error is known to
report that a function is not supported (possibly for a specific input). It
is satisfied by ErrNotSupported as well as some syscall errors.

### Func Lock
```go
func Lock(f File) error
```
Lock places an advisory write lock on the file, blocking until it can be
locked.

If Lock returns nil, no other process will be able to place a read or write
lock on the file until this process exits, closes f, or calls Unlock on it.

If f's descriptor is already read- or write-locked, the behavior of Lock is
unspecified.

Closing the file may or may not release the lock promptly. Callers should
ensure that Unlock is always called when Lock succeeds.

### Func RLock
```go
func RLock(f File) error
```
RLock places an advisory read lock on the file, blocking until it can be
locked.

If RLock returns nil, no other process will be able to place a write lock on
the file until this process exits, closes f, or calls Unlock on it.

If f is already read- or write-locked, the behavior of RLock is unspecified.

Closing the file may or may not release the lock promptly. Callers should
ensure that Unlock is always called if RLock succeeds.

### Func Unlock
```go
func Unlock(f File) error
```
Unlock removes an advisory lock placed on f by this process.

The caller must not attempt to unlock a file that is not locked.



## Types
### Type File
```go
type File interface {
	// Name returns the name of the file.
	Name() string

	// Fd returns a valid file descriptor.
	// (If the File is an *os.File, it must not be closed.)
	Fd() uintptr

	// Stat returns the FileInfo structure describing file.
	Stat() (os.FileInfo, error)
}
```
A File provides the minimal set of methods required to lock an open file.
File implementations must be usable as map keys. The usual implementation is
*os.File.





