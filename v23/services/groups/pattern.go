// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This is WIP to implement and test algorithms that match blessings
// against blessing patterns that involve groups. We expect to
// eventually migrate some of this code to v23/security/pattern.go.

package groups

import (
	"errors"
	"regexp"
	"sort"
	"strings"

	"v.io/v23/context"
	"v.io/v23/security"
	"v.io/v23/verror"
)

const (
	GroupStart = "<grp:" // GroupStart indicates the start of a group name in a blessing pattern.
	GroupEnd   = ">"     // GroupEnd indicates the end of a group name in a blessing pattern.
)

var (
	exactMatch    = convertToSet("")
	grpRegexp     = regexp.MustCompile(GroupStart + "[^" + GroupEnd + "]*" + GroupEnd)
	grpStartToken = strings.TrimSuffix(GroupStart, security.ChainSeparator)
)

// Match matches blessing names against a pattern. It returns an empty array if
// the presented blessing names do not match the pattern, or a set of suffixes
// when the pattern is a prefix of the blessing names. Match approximates the
// result if any errors are encountered during the matching process.
//
// Match assumes presented blessing names are valid. ctx is used to make any
// outgoing RPCs.
// TODO(hpucha): Enhance to add versioning, scrub Approximation content for privacy.
func Match(ctx *context.T, p security.BlessingPattern, hint ApproximationType, visitedGroups map[string]struct{}, blessings map[string]struct{}) (map[string]struct{}, []Approximation) {
	if visitedGroups == nil {
		visitedGroups = make(map[string]struct{})
	}

	g := &grpClient{
		hint:       hint,
		visited:    visitedGroups,
		rpcHandler: groupClientRPCImpl{},
	}
	return g.match(ctx, p, blessings), g.apprxs
}

type grpClient struct {
	hint       ApproximationType
	version    string
	visited    map[string]struct{}
	apprxs     []Approximation
	rpcHandler groupClientRPC
}

func (g *grpClient) match(ctx *context.T, p security.BlessingPattern, blessings map[string]struct{}) map[string]struct{} {
	if len(p) == 0 {
		return nil
	}

	// Note: The correct answer here is that the returned set must
	// consist of exactMatch and every proper suffix of the
	// blessing string. Since we do not allow any components after
	// security.AllPrincipals, returning exactMatch is sufficient.
	if p == security.AllPrincipals {
		return exactMatch
	}

	patTokens, err := splitPattern(p)
	if err != nil {
		// Approximate the result.
		errTmp := verror.ErrBadArg.Errorf(ctx, "bad argument: malformed pattern: %s", p)
		g.apprxs = append(g.apprxs, Approximation{Reason: string(verror.ErrorID(errTmp)), Details: errTmp.Error()})
		return approxRemainder(g.hint, blessings)
	}

	glob := true
	if patTokens[len(patTokens)-1] == string(security.NoExtension) {
		glob = false
		patTokens = patTokens[:len(patTokens)-1]
	}

	remainder := make(map[string]struct{})
	for b := range blessings {
		rem := g.matchedByBlessing(ctx, patTokens, b)

		if glob {
			remainder = union(remainder, rem)
			continue
		}

		for s := range rem {
			if s == "" {
				remainder = union(remainder, exactMatch)
				break
			}
		}
	}
	return remainder
}

func (g *grpClient) matchedByBlessing(ctx *context.T, patTokens []string, b string) map[string]struct{} {
	remainder := convertToSet(b)

	for _, patTok := range patTokens {

		removeEmptyStrings(remainder)
		if len(remainder) == 0 {
			return nil
		}

		if group := groupName(patTok); group != "" {
			remainder = g.remainder(ctx, group, remainder)
			continue
		}

		// Pattern token is not a group reference.
		newRemainder := make(map[string]struct{})
		for b := range remainder {
			btok, suffix := leftMostToken(b)
			if patTok == btok {
				newRemainder[suffix] = struct{}{}
			}
		}
		remainder = newRemainder
	}

	return remainder
}

func cycle(visited map[string]struct{}) string {
	grps := make([]string, len(visited))
	i := 0
	for k := range visited {
		grps[i] = k
		i++
	}
	sort.Strings(grps)
	return strings.Join(grps, " -> ")
}

func (g *grpClient) remainder(ctx *context.T, groupName string, blessingChunks map[string]struct{}) map[string]struct{} {
	var err error
	var apprxs []Approximation
	var remainder map[string]struct{}

	if _, ok := g.visited[groupName]; ok {
		err = ErrorfCycleFound(ctx, "Found cycle in group definitions %v visited %v", groupName, cycle(g.visited))
	} else {
		visited := copyMap(g.visited)
		visited[groupName] = struct{}{}
		remainder, apprxs, _, err = g.rpcHandler.relate(ctx, groupName, blessingChunks, g.hint, g.version, visited)
	}

	g.apprxs = append(g.apprxs, apprxs...)
	if err != nil {
		// For any error, suitably approximate the result.
		g.apprxs = append(g.apprxs, Approximation{Reason: string(verror.ErrorID(err)), Details: err.Error()})
		return approxRemainder(g.hint, blessingChunks)
	}

	return remainder
}

// Helper functions
// ================

// splitPattern splits a pattern into valid tokens separated by ":"
// (security.ChainSeparator). If any token in a pattern is invalid, it returns
// an error.
//
// A group token (<grp:[grpname]>) is treated as indivisible. A pattern token
// is valid if it is a valid blessing extension or a single valid group token.
// A token that concatenates two group tokens, or a group token concatenated
// with a valid blessing extension, is invalid.
//
// TODO(hpucha): Is there a better way to do this split using regex
// matching?
func splitPattern(p security.BlessingPattern) ([]string, error) {
	// Check that the blessing pattern is valid without the
	// presence of group tokens (see v23/security/pattern.go).
	if pTmp := security.BlessingPattern(grpRegexp.ReplaceAllLiteralString(string(p), "X")); !pTmp.IsValid() {
		return nil, errors.New("invalid pattern: non-group tokens are invalid")
	}

	pStr := string(p)
	var patTokens []string

	for pStr != "" {
		tokens := strings.SplitN(pStr, security.ChainSeparator, 2)

		if !strings.Contains(tokens[0], grpStartToken) {
			// Token is a blessing extension.
			patTokens = append(patTokens, tokens[0])
			if len(tokens) != 2 {
				break
			}
			pStr = tokens[1]
			continue
		}

		// Check for a valid group token.
		if !strings.HasPrefix(tokens[0], grpStartToken) {
			// Invalid group token. <grp: is not at the beginning of a token.
			return nil, errors.New("invalid pattern: group token has a prefix")
		}

		endpos := strings.Index(pStr, GroupEnd)

		if endpos < 0 {
			// Invalid group token. Matching ">" not found.
			return nil, errors.New("invalid pattern: group end delimiter not found")
		}

		if endpos < len(pStr)-1 && pStr[endpos+1] != security.ChainSeparator[0] {
			// The pattern must either end here, or be followed by a /.
			return nil, errors.New("invalid pattern: group token has a suffix")
		}

		grpName := pStr[len(GroupStart):endpos]
		if grpName == "" {
			return nil, errors.New("invalid pattern: group token is empty")
		}
		patTokens = append(patTokens, pStr[0:endpos+1])

		// Skip the / and check if any string is remaining.
		if endpos+2 >= len(pStr) {
			return patTokens, nil
		}

		pStr = pStr[endpos+2:]
	}
	return patTokens, nil
}

// groupName extracts the group name from a pattern token.
func groupName(pattok string) string {
	if strings.HasPrefix(pattok, GroupStart) && strings.HasSuffix(pattok, GroupEnd) {
		return pattok[len(GroupStart) : len(pattok)-len(GroupEnd)]
	}
	return ""
}

// leftMostToken returns the left most token and the remaining suffix
// of a blessing name.
func leftMostToken(b string) (string, string) {
	tokens := strings.SplitN(b, security.ChainSeparator, 2)
	suffix := ""
	if len(tokens) == 2 {
		suffix = tokens[1]
	}
	return tokens[0], suffix
}

// removeEmptyStrings removes any empty blessing chunks from the
// remainder set.
func removeEmptyStrings(remainder map[string]struct{}) {
	for b := range remainder {
		if b == "" {
			delete(remainder, b)
		}
	}
}

func copyMap(in map[string]struct{}) map[string]struct{} {
	out := make(map[string]struct{}, len(in))
	for k := range in {
		out[k] = struct{}{}
	}
	return out
}

func approxRemainder(hint ApproximationType, blessingChunks map[string]struct{}) map[string]struct{} {
	if hint == ApproximationTypeUnder {
		return nil
	}

	remainder := make(map[string]struct{})
	for bc := range blessingChunks {
		properSuffixes(remainder, bc)
	}
	return remainder
}

func properSuffixes(suffixes map[string]struct{}, bc string) {
	parts := strings.SplitN(bc, security.ChainSeparator, 2)
	for len(parts) == 2 {
		suffixes[parts[1]] = struct{}{}
		parts = strings.SplitN(parts[1], security.ChainSeparator, 2)
	}
	suffixes[""] = struct{}{}
}

// union merges set s2 into s1 and returns s1.
func union(s1, s2 map[string]struct{}) map[string]struct{} {
	for k := range s2 {
		s1[k] = struct{}{}
	}
	return s1
}

// groupClientRPC is the interface for the group client making the Relate RPC.
//
// Used for faking RPC handling in tests.
// TODO(hpucha/ashankar): Investigate if injecting a mock rpc.Client for tests is possible.
type groupClientRPC interface {
	relate(ctx *context.T, groupName string, blessingChunks map[string]struct{}, hint ApproximationType, version string, vGrps map[string]struct{}) (map[string]struct{}, []Approximation, string, error)
}

type groupClientRPCImpl struct{}

func (g groupClientRPCImpl) relate(ctx *context.T, groupName string, blessingChunks map[string]struct{}, hint ApproximationType, version string, vGrps map[string]struct{}) (map[string]struct{}, []Approximation, string, error) {
	client := GroupClient(groupName)
	return client.Relate(ctx, blessingChunks, hint, version, vGrps)
}
