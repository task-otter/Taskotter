// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package normalize

type (

	// CollisionError reports two source modules normalizing to the same destination.
	CollisionError struct {
		SourceA     string
		SourceB     string
		Destination string
	}

	// Mapping records a source module and its normalized destination name.
	Mapping = struct {
		Source      string
		Destination string
	}
)
