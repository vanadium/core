// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package slang provides a simple, type checked scripting language.
//
// The string constants Summary, Literal and Examples describe the language
// and are provided as exported strings to allow clients of the this package
// to display them at run-time.
package slang

const (
	// Summary is a summary of the slang language for use in
	// displaying help messages etc.
	Summary = `The language consists of a series of invocations on functions, with no control flow. Variables can only be created from the results of such invocations. Once so created they may be used as arguments to subsequent invocations. Execution stops on first error. Go-style comments are allowed. All variables are typed as per Go's type system and their use is type-checked before any functions are run.
`

	// Literals describes the literals supported by the slang
	// language for use in displaying help messages etc.
	Literals = `Literal values are supported as per Go's syntax for int's, float's, bool's, string's and time.Duration
`

	// Examples contains a simple example of the slang
	// language for use in displaying help messages etc.
	Examples = `printHelloWorld() // A function with a side-effect.

a, b := createTwoVariables() // A function that returns two variables.
printf("%v %v", a, b) // Note the use of a string literal for the format arg.
c := useA(a)
useB(b, c)
`
)
