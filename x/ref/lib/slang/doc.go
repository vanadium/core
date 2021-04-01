// Package slang provides a simple, type checked scripting language.
//
// The language consists of a series of invocations on go functions
// executed in the order they are defined with no control flow.
// This package provides a small set of builtin functions and additional
// functions can be registered using RegisterFunction. The results of an
// invocation can create new variables that can be used in subesequent
// invocations. There are no other means of creating variables.
// A slang script is type checked whilst being compiled to an
// intermediate format before being executed. Execution uses
// go's reflect package to invoke one of the predefined functions.
package slang
