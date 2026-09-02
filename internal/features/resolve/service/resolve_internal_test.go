// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service

import (
	"testing"

	"github.com/task-otter/Taskotter/internal/shared/consts"
)

const (
	scoreFmt   = "score = %d, want %d"
	goTask     = "go"
	golangName = "golang"
)

// TestCompareScoredCandidatesOrdersByScoreThenName verifies ranking is stable and descending.
func TestCompareScoredCandidatesOrdersByScoreThenName(t *testing.T) {
	t.Parallel()

	high := scoredCandidate{name: "alpha", score: consts.IndexTwo}
	low := scoredCandidate{name: "beta", score: consts.IndexOne}
	tie := scoredCandidate{name: "zulu", score: consts.IndexTwo}

	assertCompare(t, compareScoredCandidates(high, low), -consts.IndexOne)
	assertCompare(t, compareScoredCandidates(low, high), consts.IndexOne)
	assertCompare(t, compareScoredCandidates(high, tie), -consts.IndexOne)
}

// TestSimilarityScoresExactAndPrefixMatches verifies the fast-path scores.
func TestSimilarityScoresExactAndPrefixMatches(t *testing.T) {
	t.Parallel()

	if got := similarity(goTask, goTask); got != scoreExactMatch {
		t.Fatalf(scoreFmt, got, scoreExactMatch)
	}

	want := scorePrefixMatchBase + len(goTask)

	if got := similarity(goTask, golangName); got != want {
		t.Fatalf(scoreFmt, got, want)
	}
}

// TestLevenshteinHandlesEmptyAndIdenticalInputs verifies the boundary scores.
func TestLevenshteinHandlesEmptyAndIdenticalInputs(t *testing.T) {
	t.Parallel()

	if got := levenshtein(goTask, goTask); got != scoreIdenticalString {
		t.Fatalf(scoreFmt, got, scoreIdenticalString)
	}

	if got := levenshtein(consts.Empty, goTask); got != consts.IndexZero {
		t.Fatalf(scoreFmt, got, consts.IndexZero)
	}

	if got := levenshtein(golangName, "rust"); got < consts.IndexZero {
		t.Fatalf("score = %d, want non-negative", got)
	}
}

func assertCompare(t *testing.T, got, want int) {
	t.Helper()

	if got != want {
		t.Fatalf(scoreFmt, got, want)
	}
}
