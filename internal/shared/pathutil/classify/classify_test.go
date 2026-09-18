// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package classify

import (
	"testing"
)

// TestClassifiers verifies path classification helpers.
func TestClassifiers(t *testing.T) {
	tests := []struct {
		name string
		got  bool
		want bool
	}{
		{name: "readme", got: IsDocPath("README.md"), want: true},
		{name: "nested docs", got: IsDocPath("a/docs/guide.md"), want: true},
		{name: "metadata", got: IsModuleMetadataPath("metadata.yml"), want: true},
		{name: "test file", got: IsTestPath("pkg/file_test.go"), want: true},
		{
			name: "folder child",
			got:  HasFolderPrefix("taskfiles/go/Taskfile.yml", "taskfiles"),
			want: true,
		},
	}

	for _, test := range tests {
		if test.got != test.want {
			t.Errorf("%s = %t, want %t", test.name, test.got, test.want)
		}
	}
}
