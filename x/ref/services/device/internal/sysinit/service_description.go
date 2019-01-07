// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sysinit

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"text/template"
	"time"

	"v.io/v23/verror"
)

const pkgPath = "v.io/x/ref/services/device/internal/sysinit"

var (
	errMarshalFailed   = verror.Register(pkgPath+".errMarshalFailed", verror.NoRetry, "{1:}{2:} Marshal({3}) failed{:_}")
	errWriteFileFailed = verror.Register(pkgPath+".errWriteFileFailed", verror.NoRetry, "{1:}{2:} WriteFile({3}) failed{:_}")
	errReadFileFailed  = verror.Register(pkgPath+".errReadFileFailed", verror.NoRetry, "{1:}{2:} ReadFile({3}) failed{:_}")
	errUnmarshalFailed = verror.Register(pkgPath+".errUnmarshalFailed", verror.NoRetry, "{1:}{2:} Unmarshal({3}) failed{:_}")
)

const dateFormat = "Jan 2 2006 at 15:04:05 (MST)"

// ServiceDescription is a generic service description that represents the
// common configuration details for specific systems.
type ServiceDescription struct {
	Service     string            // The name of the Service
	Description string            // A description of the Service
	Environment map[string]string // Environment variables needed by the service
	Binary      string            // The binary to be run
	Command     []string          // The script/binary and command line options to use to start/stop the binary
	User        string            // The username this service is to run as
}

// TODO(caprita): Unit test.

// SaveTo serializes the service description object to a file.
func (sd *ServiceDescription) SaveTo(fName string) error {
	jsonSD, err := json.Marshal(sd)
	if err != nil {
		return verror.New(errMarshalFailed, nil, sd, err)
	}
	if err := ioutil.WriteFile(fName, jsonSD, 0600); err != nil {
		return verror.New(errWriteFileFailed, nil, fName, err)
	}
	return nil
}

// LoadFrom de-serializes the service description object from a file created by
// SaveTo.
func (sd *ServiceDescription) LoadFrom(fName string) error {
	if sdBytes, err := ioutil.ReadFile(fName); err != nil {
		return verror.New(errReadFileFailed, nil, fName, err)
	} else if err := json.Unmarshal(sdBytes, sd); err != nil {
		return verror.New(errUnmarshalFailed, nil, sdBytes, err)
	}
	return nil
}

func (sd *ServiceDescription) writeTemplate(templateContents, file string) error {
	conf, err := template.New(sd.Service + ".template").Parse(templateContents)
	if err != nil {
		return err
	}
	w := os.Stdout
	if len(file) > 0 {
		w, err = os.Create(file)
		if err != nil {
			return err
		}
	}
	type tmp struct {
		*ServiceDescription
		Date string
	}
	data := &tmp{
		ServiceDescription: sd,
		Date:               time.Now().Format(dateFormat),
	}
	return conf.Execute(w, &data)
}
