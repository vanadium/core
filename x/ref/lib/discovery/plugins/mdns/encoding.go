// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mdns

import (
	"encoding/base32"
	"errors"
	"regexp"
	"sort"
	"strings"

	"v.io/v23/discovery"
)

const (
	// Limit the maximum large txt records to the maximum total txt records size.
	maxLargeTxtRecordLen = maxTotalTxtRecordsLen

	// The index of large txt records ranges from '0' to '~' as defined in
	// reLargeTxtRecord.
	maxNumLargeTxtRecords = '~' - '0' + 1
)

var (
	// The key of encoded large txt records is "_x<i><j>", where 'i' and 'j' are
	// one byte indices. This should be enough with the current limit on the
	// advertisement size and the number of attributes or attachments.
	reLargeTxtRecord = regexp.MustCompile("^" + attrLargeTxtPrefix + "[0-~][0-~]=")
)

// encodeAdID encodes the given advertisement id to a valid host name by using
// "Extended Hex Alphabet" defined in RFC 4648. This removes any padding characters.
func encodeAdID(id *discovery.AdId) string {
	return strings.TrimRight(base32.HexEncoding.EncodeToString(id[:]), "=")
}

// decodeAdID decodes the given host name to the advertisement id.
func decodeAdID(hostname string, id *discovery.AdId) error {
	// Add padding characters if needed.
	if p := len(hostname) % 8; p > 0 {
		hostname += strings.Repeat("=", 8-p)
	}

	decoded, err := base32.HexEncoding.DecodeString(hostname)
	if err != nil {
		return err
	}
	if len(decoded) != len(id) {
		return errors.New("invalid hostname")
	}
	copy(id[:], decoded)
	return nil
}

// maybeSplitTxtRecord slices a txt record into multiple records if it is larger than 255 bytes.
func maybeSplitTxtRecord(txt string, xseq int) ([]string, int) {
	n := len(txt)
	if n <= maxTxtRecordLen {
		return []string{txt}, xseq
	}

	// The overhead of a large txt record is 5 bytes -
	// "_x<i><j>=", where 'i' and 'j' are one byte.
	const overhead = 5
	splitted := make([]string, 0, (n+maxTxtRecordLen-overhead-1)/(maxTxtRecordLen-overhead))
	var buf [maxTxtRecordLen]byte
	copy(buf[:], attrLargeTxtPrefix)
	for i, off := 0, 0; off < n; i++ {
		buf[2] = byte(xseq + '0')
		buf[3] = byte(i + '0')
		buf[4] = '='
		c := copy(buf[5:], txt[off:])
		splitted = append(splitted, string(buf[:5+c]))
		off += c
	}
	return splitted, xseq + 1
}

// maybeJoinTxtRecords joins the splitted large txt records.
func maybeJoinTxtRecords(txts []string) ([]string, error) {
	joined, splitted := make([]string, 0, len(txts)), make([]string, 0, len(txts))
	for _, txt := range txts {
		switch {
		case strings.HasPrefix(txt, attrLargeTxtPrefix):
			if !reLargeTxtRecord.MatchString(txt) {
				return nil, errors.New("invalid large txt record")
			}
			splitted = append(splitted, txt)
		default:
			joined = append(joined, txt)
		}
	}
	if len(splitted) == 0 {
		return joined, nil
	}

	sort.Strings(splitted)

	var buf [maxLargeTxtRecordLen]byte
	xseq, off := -1, 0
	for _, txt := range splitted {
		i := int(txt[2] - '0')
		if i > xseq {
			if off > 0 {
				// A new large txt record started.
				joined = append(joined, string(buf[:off]))
			}
			xseq = i
			off = 0
		}
		c := copy(buf[off:], txt[5:])
		off += c
	}
	joined = append(joined, string(buf[:off]))
	return joined, nil
}
