// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service_test

import (
	"testing"

	storesvc "github.com/task-otter/Taskotter/internal/features/store/service"
)

func BenchmarkLoadCatalogAndDeps(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _, err := storesvc.LoadCatalogAndDeps(fixtureStoreRoot)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLocalSnapshot(b *testing.B) {
	ref := fixtureRefInfo()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := storesvc.LocalSnapshot(fixtureStoreRoot, ref)
		if err != nil {
			b.Fatal(err)
		}
	}
}
