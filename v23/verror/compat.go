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
// verror.SubErrors followed by any error recorded with %w for Errorf or the
// last argument to Message, New or ExplicitNew.
func (e E) Unwrap() error {
	r := []error{}
	for _, p := range e.ParamList {
		switch se := p.(type) {
		case SubErrs:
			for _, s := range se {
				r = append(r, s)
			}
		case SubErr:
			r = append(r, se)
		case *SubErr:
			r = append(r, se)
		}
	}
	if e.unwrap != nil {
		r = append(r, e.unwrap)
	}
	if len(r) == 0 {
		return nil
	}
	return subErrChain{
		err:  r[0],
		rest: r[1:],
	}
}
