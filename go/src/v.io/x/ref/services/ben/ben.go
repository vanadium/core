// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ben

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
	"unicode"
)

// ID returns a short identifier for Code.
func (c SourceCode) ID() string {
	s := string(c)
	if len(s) <= 32 && len(s) > 0 && strings.IndexFunc(s, unicode.IsSpace) == -1 {
		return s
	}
	hash := sha256.Sum256([]byte(s))
	return hex.EncodeToString(hash[:])
}

// PrettyTime returns a string representing r.NanoSecsPerOp in a more
// human-friendly form, similar to time.Duration.String.
func (r Run) PrettyTime() string {
	if r.NanoSecsPerOp < 100 {
		return fmt.Sprintf("%vns", r.NanoSecsPerOp)
	}
	return time.Duration(int64(r.NanoSecsPerOp)).String()
}
