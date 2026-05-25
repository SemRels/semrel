// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package releasenotes

import (
	"github.com/GoSemantics/semrel/pkg/commits"
)

// Build constructs a ReleaseNotes from a list of parsed commits.
// It classifies each commit into the appropriate section.
func Build(version string, cs []*commits.Commit) *ReleaseNotes {
	rn := &ReleaseNotes{Version: version}
	for _, c := range cs {
		if c == nil {
			continue
		}
		e := Entry{
			Type:        c.Type,
			Scope:       c.Scope,
			Description: c.Description,
			IsBreaking:  c.IsBreakingChange,
		}
		switch {
		case c.IsBreakingChange:
			rn.Breaking = append(rn.Breaking, e)
		case c.Type == "feat":
			rn.Features = append(rn.Features, e)
		case c.Type == "fix" || c.Type == "perf" || c.Type == "revert":
			rn.Fixes = append(rn.Fixes, e)
		case c.Type != "":
			rn.Others = append(rn.Others, e)
		}
	}
	return rn
}
