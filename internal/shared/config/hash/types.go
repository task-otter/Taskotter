// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package hash

// Input is the canonical configuration payload used for branch identity.
type (
	// Input is the canonical configuration payload used for branch identity.
	Input struct {
		NodePackageManager string   `json:"node_package_manager"`
		StoreVersion       string   `json:"store_version"`
		TargetFolder       string   `json:"target_folder"`
		Tasks              []string `json:"tasks"`
		IncludesDoc        bool     `json:"includes_doc"`
		SyncRoot           bool     `json:"sync_root"`
	}
)
