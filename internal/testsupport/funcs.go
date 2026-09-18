// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

// Package testsupport provides shared testing helpers.
package testsupport

// Lock serializes tests that replace process-wide seams or environment state.
func Lock() func() {
	testStateMu.Lock()

	return testStateMu.Unlock
}
