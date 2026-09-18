// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package cli

import (
	"context"
)

type (
	pathSet = map[string]struct{}

	credArgs = struct {
		token      string
		repository string
	}

	clientFns = struct {
		run       func(ctx context.Context, args ...string) error
		output    func(ctx context.Context, args ...string) (string, error)
		binary    string
		workspace string
	}

	// Client runs git commands in a workspace directory.
	Client struct {
		fns clientFns
	}
)
