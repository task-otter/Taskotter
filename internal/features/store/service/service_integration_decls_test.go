// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service_test

type (
	// fixtureStore holds the catalog and dependency graph of the fixture store.
	fixtureStore struct {
		catalog map[string]struct{}
		deps    map[string][]string
	}
)

const (
	fixtureStoreRoot     = "../../../../tests/fixtures/store"
	moduleESLintNodePnpm = "eslint/node/pnpm"
	moduleESLint         = "eslint"
	noDependencies       = 0
)
