// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package semver

import "testing"

func FuzzParseVersion(f *testing.F) {
	f.Add("1.0.0")
	f.Add("v1.2.3")
	f.Add("0.0.0")
	f.Add("1.0.0-alpha.1")
	f.Add("999.999.999")
	f.Fuzz(func(t *testing.T, s string) {
		_, _ = ParseVersion(s)
	})
}
