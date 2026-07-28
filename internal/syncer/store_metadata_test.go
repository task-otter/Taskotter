package syncer

import (
	"path/filepath"
	"testing"

	"github.com/mostafakhairy0305-dot/TaskOtter/internal/config"
	"github.com/mostafakhairy0305-dot/TaskOtter/internal/store"
)

func TestStoreTaskMetadataVariantInheritsParent(t *testing.T) {
	t.Parallel()

	snap := fixtureSnapshot(t)
	metadata, err := loadStoreTaskMetadata(snap)
	if err != nil {
		t.Fatal(err)
	}

	meta, ok := resolveStoreTaskMetadata("eslint/node/fnm/pnpm", metadata)
	if !ok {
		t.Fatal("expected inherited eslint metadata")
	}

	if meta.Module != "eslint" {
		t.Fatalf("Module = %q, want eslint", meta.Module)
	}
}

func TestBuildGeneratedRootTasksRequestedOnly(t *testing.T) {
	t.Parallel()

	snap := fixtureSnapshot(t)
	metadata, err := loadStoreTaskMetadata(snap)
	if err != nil {
		t.Fatal(err)
	}

	generated := buildGeneratedRootTasks(
		[]string{"go", "eslint"},
		map[string]ModuleRecord{
			"go": {
				SourceModule:      "go",
				DestinationModule: "go",
				Path:              "taskfiles/go",
			},
			"eslint": {
				SourceModule:      "eslint/node/fnm/pnpm",
				DestinationModule: "eslint",
				Path:              "taskfiles/eslint",
			},
		},
		metadata,
	)

	got := generatedRootTaskNames(generated)
	want := []string{"install", "lint", "lint:fix", "version"}
	if len(got) != len(want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %#v, want %#v", got, want)
		}
	}
}

func fixtureSnapshot(t *testing.T) *store.Snapshot {
	t.Helper()

	root := filepath.Join("..", "..", "tests", "fixtures", "store")
	snap, err := store.LocalSnapshot(root, store.RefInfo{
		Repository:    config.StoreRepository,
		DefaultBranch: "main",
	})
	if err != nil {
		t.Fatal(err)
	}

	return snap
}
