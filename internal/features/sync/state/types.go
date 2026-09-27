// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package state

type (
	loadStateRequest[value any] struct {
		decode    func([]byte, *value) error
		workspace string
		rel       string
		label     string
	}
)
