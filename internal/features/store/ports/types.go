// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package ports

import (
	"net/http"
)

type (
	// HTTPDoer performs HTTP requests for the store client.
	HTTPDoer interface {
		Do(req *http.Request) (*http.Response, error)
	}
)
