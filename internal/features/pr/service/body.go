// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

// Package service builds TaskOtter sync pull request bodies.
package service

import (
	"strings"

	"github.com/task-otter/Taskotter/internal/features/pr/domain"
	storedomain "github.com/task-otter/Taskotter/internal/features/store/domain"
	syncdomain "github.com/task-otter/Taskotter/internal/features/sync/domain"
	"github.com/task-otter/Taskotter/internal/features/sync/domain/lockmodel"
	"github.com/task-otter/Taskotter/internal/shared/config"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
)

const (
	fmtBulletItem = "  - `%s`\n"
)

// StoreRefFrom maps store ref info into a PR body store ref.
func StoreRefFrom(ref *storedomain.RefInfo) *domain.StoreRef {
	return &domain.StoreRef{
		SourceRef:      ref.SourceRef,
		ResolvedCommit: ref.ResolvedCommit,
		DefaultBranch:  ref.DefaultBranch,
	}
}

// BuildPRBody renders the markdown body for a sync pull request.
func BuildPRBody(cfg *config.Config, plan *syncdomain.Plan, ref *domain.StoreRef) string {
	var body strings.Builder

	builderWriteString(&body, "## TaskOtter\n\n")

	writeMetadataSection(&body, cfg, ref)
	writeRequestedModulesSection(&body, cfg, plan)
	writeDependenciesSection(&body, plan.Dependencies)
	writeFileChangesSection(&body, plan)

	return body.String()
}

// builderWriteString appends text to body. Writes to a [strings.Builder] cannot
// fail, so the error is consumed here.
func builderWriteString(body *strings.Builder, text string) {
	iox.Discard(iox.WriteStringFull(body, text))
}

// builderPrintf appends formatted text to body. Writes to a [strings.Builder]
// cannot fail, so the error is consumed here.
func builderPrintf(body *strings.Builder, format string, args ...any) {
	iox.Discard(iox.Fprintf(body, format, args...))
}

func writeMetadataSection(body *strings.Builder, cfg *config.Config, ref *domain.StoreRef) {
	writeCoreMetadata(body, cfg, ref)
	writeJSRuntimeMetadata(body, cfg)

	builderWriteString(body, consts.Newline)
}

func writeCoreMetadata(body *strings.Builder, cfg *config.Config, ref *domain.StoreRef) {
	builderPrintf(body, "- Source: `%s`\n", config.StoreRepository)
	builderPrintf(body, "- Requested version: `%s`\n", emptyDash(cfg.StoreVersion))
	builderPrintf(body, "- Source reference: `%s`\n", ref.SourceRef)
	builderPrintf(body, "- Resolved commit: `%s`\n", ref.ResolvedCommit)
	builderPrintf(body, "- Default branch: `%s`\n", ref.DefaultBranch)
	builderPrintf(body, "- Target folder: `%s`\n", cfg.TargetFolder)
	builderPrintf(body, "- Documentation included: `%t`\n", cfg.IncludesDoc)
	builderPrintf(body, "- Root Taskfile synchronized: `%t`\n", cfg.SyncRoot)
	builderPrintf(body, "- JS runtime: `%s`\n", emptyDash(string(cfg.JSRuntime)))
}

func writeJSRuntimeMetadata(body *strings.Builder, cfg *config.Config) {
	if cfg.JSRuntime != config.JSRuntimeNodeJS {
		return
	}

	builderPrintf(body, "- Package manager: `%s`\n", cfg.NodePackageManager)
}

func writeRequestedModulesSection(
	body *strings.Builder,
	cfg *config.Config,
	plan *syncdomain.Plan,
) {
	builderWriteString(body, "### Requested modules\n\n")
	builderWriteString(body, "| Task | Source module | Destination |\n")
	builderWriteString(body, "|---|---|---|\n")

	tasks := cfg.Tasks

	for idx := range tasks {
		task := tasks[idx]
		rec := plan.Requested[task]
		builderPrintf(body, "| %s | `%s` | `%s` |\n", task, rec.SourceModule, rec.Path)
	}
}

func writeDependenciesSection(body *strings.Builder, deps []lockmodel.ModuleRecord) {
	builderWriteString(body, "\n### Dependencies\n\n")
	builderWriteString(body, "| Source module | Destination |\n")
	builderWriteString(body, "|---|---|\n")

	for idx := range deps {
		dep := &deps[idx]
		builderPrintf(body, "| `%s` | `%s` |\n", dep.SourceModule, dep.Path)
	}
}

func writeFileChangesSection(body *strings.Builder, plan *syncdomain.Plan) {
	builderWriteString(body, "\n### File changes\n\n")
	writeBulletGroup(body, "Added", plan.Added)
	writeBulletGroup(body, "Updated", plan.Updated)
	writeBulletGroup(body, "Removed", plan.Removed)
}

func writeBulletGroup(body *strings.Builder, label string, paths []string) {
	builderPrintf(body, "- %s: %d\n", label, len(paths))

	for idx := range paths {
		builderPrintf(body, fmtBulletItem, paths[idx])
	}
}

func emptyDash(value string) string {
	if value == consts.Empty {
		return consts.Empty
	}

	return value
}
