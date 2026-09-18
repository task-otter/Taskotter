// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package managed

type (
	// File tracks a synced file in the lock file.
	File = struct {
		SourceModule      string `yaml:"source_module"`
		DestinationModule string `yaml:"destination_module"`
		SourcePath        string `yaml:"source_path"`
		Path              string `yaml:"path"`
		SHA256            string `yaml:"sha256"`
	}

	yamlDecodeTarget = struct {
		Out any
		Key string
	}
)
