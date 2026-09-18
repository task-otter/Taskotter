// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package faults

type (
	// StubReader reports a fixed error, or ErrFault when Err is nil.
	StubReader struct {
		Err error
	}

	// StubWriter reports Count bytes written and Err, letting tests drive short,
	// negative, and failing write branches. A nil Err reports success.
	StubWriter struct {
		Err   error
		Count int
	}
)
