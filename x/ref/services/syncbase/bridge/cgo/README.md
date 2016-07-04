# Syncbase cgo bridge

The Syncbase cgo bridge exposes a C API to Syncbase. It's intended to be used on
iOS (Swift) and Android (Java), and perhaps elsewhere as well.

For the time being, we provide a `Makefile` to build C archives for all target
platforms. Eventually, we'll integrate C archive compilation with the `jiri`
tool.

# JNI

Java expects functions that have names and arguments that match the `native`
declarations from the `.java` files located in `java/io/v/syncbase/internal`.

Layout:

- `jni.go`: the JNI functions expected by Java.
- `jni_util.go`: helper functions to find Java classes and retrieve IDs for
   object fields and methods.
- `jni_lib.go`: types and functions to cache the field and method IDs for all
   the classes the code needs to touch.
- `jni_types.go`: methods and functions to convert between Java and the types
   declared in `lib.h`; counterpart of `types.go`.
- `jni_wrapper.{h,c}`: trampoline functions that compensate for the fact that Go
   cannot call a function pointer.

Some design notes:

- looking up classes/interfaces, methods and field IDs is expensive. The global
  variables named like `fooClass` from `jni.go` cache all we need for a class
  `foo`. The load is done in `JNI_OnLoad`, a function that is called each time
  the our library is loaded (typically, using `System.loadLibrary()`).
- some of the `extractToJava()` methods and `newVFoo()` functions are in
  `jni.go` instead of `jni_types.go` to avoid some
  _"inconsistent definitions"_ errors.
- the `extractToJava()` methods convert from low-level types like
  `v23_syncbase_Foo` to their Java equivalents. The `extract()` methods convert
  to Go equivalents. Both `extract()` and `extractToJava()` will free all the
  pointers inside the primitive types they are extracting from.
- the `free()` methods are idempotent.
- we use `panic()` to enforce certain invariants (enough memory, existence of
  classes, methods, fields, etc) and to keep the code simple.