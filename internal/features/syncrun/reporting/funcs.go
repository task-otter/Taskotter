// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package reporting

import (
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"

	rundomain "github.com/task-otter/Taskotter/internal/features/syncrun/domain"
	"github.com/task-otter/Taskotter/internal/shared/config"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
)

// ReportSyncRequired writes GitHub Actions annotations when a sync pull request must be merged.
func ReportSyncRequired(result *rundomain.Result) {
	ReportSyncRequiredTo(os.Stderr, result)
}

// ReportSyncRequiredTo writes sync-required GitHub Actions annotations to writer.
func ReportSyncRequiredTo(writer io.Writer, result *rundomain.Result) {
	writeSyncRequiredAnnotations(writer, syncRequiredSummary(result))
}

// ReportSyncUpToDate writes GitHub Actions notices when managed files already match the store.
func ReportSyncUpToDate(result *rundomain.Result) {
	iox.FprintBestEffort(os.Stdout, syncUpToDateNotice)
	iox.FprintfBestEffortf(os.Stdout, "Store source SHA: %s\n", result.SourceSHA)
}

// SyncRequired reports whether the sync run changed managed files.
func SyncRequired(result *rundomain.Result) bool {
	return result.Changed
}

// WriteActionOutputs writes sync result fields to GitHub Actions output or stdout.
func WriteActionOutputs(cfg *config.Config, result *rundomain.Result) error {
	values := buildOutputValues(result)

	if cfg.GitHubOutput == consts.Empty {
		printOutputsToStdout(values)

		return nil
	}

	err := iox.WriteGitHubOutputs(cfg.GitHubOutput, values)
	if err != nil {
		return fmt.Errorf("write GitHub Actions outputs: %w", err)
	}

	return nil
}

func buildOutputValues(result *rundomain.Result) map[string]string {
	return map[string]string{
		"changed":               strconv.FormatBool(result.Changed),
		"store-version":         result.StoreVersion,
		"source-ref":            result.SourceRef,
		"source-sha":            result.SourceSHA,
		"target-folder":         result.TargetFolder,
		"resolved-tasks":        result.ResolvedTasksJSON,
		"resolved-dependencies": result.ResolvedDependencies,
		"pull-request-number":   result.PullRequestNumber,
		"pull-request-url":      result.PullRequestURL,
	}
}

func printOutputsToStdout(values map[string]string) {
	keys := make([]string, consts.IndexZero, len(values))

	for key := range values {
		keys = append(keys, key)
	}

	slices.Sort(keys)

	for i := range keys {
		key := keys[i]
		iox.FprintfBestEffortf(os.Stdout, "%s=%s\n", key, values[key])
	}
}

func syncRequiredSummary(result *rundomain.Result) string {
	if result.PullRequestURL == consts.Empty {
		return "TaskOtter synced taskfile changes but did not return a pull request URL."
	}

	prNumber := result.PullRequestNumber

	if prNumber == consts.Empty {
		prNumber = "unknown"
	}

	return fmt.Sprintf("TaskOtter opened sync PR #%s: %s", prNumber, result.PullRequestURL)
}

func writeSyncRequiredAnnotations(writer io.Writer, summary string) {
	iox.FprintfBestEffortf(
		writer,
		"::error title=TaskOtter sync required::%s"+syncRequiredErrorSuffix,
		summary,
	)
	iox.FprintBestEffort(writer, syncRequiredNotice)
}
