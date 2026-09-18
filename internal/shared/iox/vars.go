// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package iox

import (
	"errors"
)

var (
	errShortWrite           = errors.New("short write")
	errInvalidFprintfCount  = errors.New("invalid fprintf count")
	errInvalidFprintCount   = errors.New("invalid fprint count")
	errInvalidFprintlnCount = errors.New("invalid fprintln count")
)
