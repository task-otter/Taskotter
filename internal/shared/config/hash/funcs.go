// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package hash

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

const (
	zeroPrefixLength = 0
)

// Compute returns the full SHA-256 hex digest and its TaskOtter sync branch.
func Compute(input *Input, prefixLength int) (fullDigest, branch string, err error) {
	var buffer bytes.Buffer

	encoder := json.NewEncoder(&buffer)

	if err := encoder.Encode(input); err != nil {
		return "", "", fmt.Errorf("encode configuration input: %w", err)
	}

	data := bytes.TrimSuffix(buffer.Bytes(), []byte{'\n'})

	digest := sha256.Sum256(data)
	full := hex.EncodeToString(digest[:])

	if prefixLength < zeroPrefixLength || prefixLength > len(full) {
		return "", "", fmt.Errorf("%w: %d", ErrInvalidPrefixLength, prefixLength)
	}

	return full, "taskotter/sync-" + full[:prefixLength], nil
}
