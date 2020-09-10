// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package verror

// Error implements error.
func (id IDAction) Error() string {
	return string(id.ID)
}

// Is implements the Is method as expected by errors.Is.
func (id IDAction) Is(err error) bool {
	if verr, ok := assertIsE(err); ok {
		return id.ID == verr.ID
	}
	if iderr, ok := err.(IDAction); ok {
		return id.ID == iderr.ID
	}
	return false
}

// Is implements the Is method as expected by errors.Is.
func (e E) Is(err error) bool {
	if verr, ok := assertIsE(err); ok {
		return e.ID == verr.ID
	}
	if iderr, ok := err.(IDAction); ok {
		return e.ID == iderr.ID
	}
	return false
}

// Error implements error.
func (subErr SubErr) Error() string {
	return subErr.String()
}

// subErrChain is used to implement Unwrap.
type subErrChain struct {
	err  error
	rest []error
}

// Error implements error.
func (se subErrChain) Error() string {
	return se.err.Error()
}

// Unwrap implements the Unwrap method as expected by errors.Unwrap.
func (se subErrChain) Unwrap() error {
	if len(se.rest) == 0 {
		return nil
	}
	return subErrChain{
		err:  se.rest[0],
		rest: se.rest[1:],
	}
}

// Unwrap implements the Unwrap method as expected by errors.Unwrap. It returns
// both chained errors (ie. those passed as arguments to New or ExplicitNew)
// and SubErr's added via AddSubErr.
func (e E) Unwrap() error {
	size := 0
	for i := range e.ParamList {
		if subErrsParam, isSubErrs := e.ParamList[i].(SubErrs); isSubErrs {
			size += len(subErrsParam)
		}
		if _, isChainedErr := e.ParamList[i].(error); isChainedErr {
			size++
		}
	}
	if size != 0 {
		r := make([]error, 0, size)
		for i := range e.ParamList {
			if subErrsParam, isSubErrs := e.ParamList[i].(SubErrs); isSubErrs {
				for _, subErr := range subErrsParam {
					r = append(r, subErr)
				}
			}
			if chainedErr, isChainedErr := e.ParamList[i].(error); isChainedErr {
				r = append(r, chainedErr)

			}
		}
		return subErrChain{
			err:  r[0],
			rest: r[1:],
		}
	}
	return nil
}
