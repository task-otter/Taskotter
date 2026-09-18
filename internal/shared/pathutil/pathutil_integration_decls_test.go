// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package pathutil_test

type (
	boolCase struct {
		path string
		want bool
	}

	boolAssert struct {
		fn    func(string) bool
		name  string
		cases []boolCase
	}

	folderPrefixCase struct {
		path   string
		folder string
		want   bool
	}

	taskfileFixture struct {
		root string
		rel  string
		data []byte
	}
)
