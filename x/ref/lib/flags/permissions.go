// Copyright 2018 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package flags

import (
	"encoding/json"
	"fmt"
	"strings"

	"v.io/v23/verror"
)

// PermissionsFlag represents a flag.Value for --v23.permissions.file
type PermissionsFlag struct {
	isSet bool
	files map[string]string
}

// String implements flag.Value.
func (permsf *PermissionsFlag) String() string {
	return fmt.Sprintf("%v", permsf.files)
}

// Set implements flag.Value.
func (permsf *PermissionsFlag) Set(v string) error {
	if !permsf.isSet {
		// override the default value
		permsf.isSet = true
		permsf.files = make(map[string]string)
	}
	parts := strings.SplitN(v, ":", 2)
	if len(parts) != 2 {
		return verror.New(errNotNameColonFile, nil, v)
	}
	name, file := parts[0], parts[1]
	permsf.files[name] = file
	return nil
}

// PermissionsLiteralFlag represents a flag.Value for --v23.permissions.literal
type PermissionsLiteralFlag struct {
	isSet       bool
	permissions string
}

// String implements flag.Value.
func (permsl *PermissionsLiteralFlag) String() string {
	return fmt.Sprintf("%v", permsl.permissions)
}

// Set implements flag.Value.
func (permsl *PermissionsLiteralFlag) Set(v string) error {
	if !json.Valid([]byte(v)) {
		return fmt.Errorf("invalid json: %v", v)
	}
	permsl.isSet = true
	permsl.permissions += v
	return nil
}

// PermissionsFlags contains the values of the PermissionsFlags flag group.
type PermissionsFlags struct {
	// List of named Permissions files.
	files PermissionsFlag

	// Literal permissions, override everything else.
	literal PermissionsLiteralFlag
}

// PermissionsFile returns the file which is presumed to contain Permissions
// information associated with the supplied name parameter.
func (af PermissionsFlags) PermissionsFile(name string) string {
	return af.files.files[name]
}

// PermissionsNamesAndFiles returns the set of permission names and associated
// files specified using --v23.permissions.file.
func (af PermissionsFlags) PermissionsNamesAndFiles() map[string]string {
	if af.files.files == nil {
		return nil
	}
	r := make(map[string]string, len(af.files.files))
	for k, v := range af.files.files {
		r[k] = v
	}
	return r
}

// PermissionsLiteral returns the in-line literal permissions provided
// on the command line.
func (af PermissionsFlags) PermissionsLiteral() string {
	return af.literal.String()
}

// AddPermissionsFile adds a permissions file, which must be in
// the same format as accepted by --v23.permissions.file
func (af PermissionsFlags) AddPermissionsFile(arg string) error {
	return af.files.Set(arg)
}

// AddPermissionsLiteral adds another literal permissions statement.
func (af PermissionsFlags) AddPermissionsLiteral(arg string) error {
	return af.literal.Set(arg)
}
