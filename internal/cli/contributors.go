// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package cli

import (
	"sort"
	"strings"

	gitpkg "github.com/SemRels/semrel/pkg/git"
)

type contributorMetadata struct {
	Name              string `json:"name"`
	Email             string `json:"email"`
	Commits           int    `json:"commits"`
	FirstContribution bool   `json:"firstContribution"`
}

func buildContributorMetadata(commits []gitpkg.Commit, fullHistoryCounts map[string]int) []contributorMetadata {
	byEmail := make(map[string]*contributorMetadata, len(commits))
	for _, commit := range commits {
		email := strings.TrimSpace(commit.AuthorEmail)
		if email == "" {
			continue
		}
		key := strings.ToLower(email)
		meta, ok := byEmail[key]
		if !ok {
			meta = &contributorMetadata{
				Name:  strings.TrimSpace(commit.AuthorName),
				Email: email,
			}
			byEmail[key] = meta
		}
		if meta.Name == "" {
			meta.Name = strings.TrimSpace(commit.AuthorName)
		}
		meta.Commits++
	}

	contributors := make([]contributorMetadata, 0, len(byEmail))
	for key, meta := range byEmail {
		meta.FirstContribution = fullHistoryCounts[key] > 0 && fullHistoryCounts[key] == meta.Commits
		contributors = append(contributors, *meta)
	}

	sort.Slice(contributors, func(i, j int) bool {
		if contributors[i].Commits != contributors[j].Commits {
			return contributors[i].Commits > contributors[j].Commits
		}
		if !strings.EqualFold(contributors[i].Name, contributors[j].Name) {
			return strings.ToLower(contributors[i].Name) < strings.ToLower(contributors[j].Name)
		}
		return strings.ToLower(contributors[i].Email) < strings.ToLower(contributors[j].Email)
	})

	return contributors
}
