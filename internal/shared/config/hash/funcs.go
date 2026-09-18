// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package hash

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/task-otter/Taskotter/internal/shared/iox"
)

const zeroPrefixLength = 0

// Compute returns the full SHA-256 hex digest and its TaskOtter sync branch.
func Compute(input *Input, prefixLength int) (fullDigest, branch string, err error) {
	data, err := json.Marshal(input)
	iox.Discard(err)

	digest := sha256.Sum256(data)
	full := hex.EncodeToString(digest[:])

	if prefixLength < zeroPrefixLength || prefixLength > len(full) {
		return "", "", fmt.Errorf("%w: %d", ErrInvalidPrefixLength, prefixLength)
	}

	return full, "taskotter/sync-" + full[:prefixLength], nil
}
