// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package app

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"

	"github.com/task-otter/Taskotter/internal/config"
	"github.com/task-otter/Taskotter/internal/consts"
	"github.com/task-otter/Taskotter/internal/github"
	"github.com/task-otter/Taskotter/internal/logging"
	"github.com/task-otter/Taskotter/internal/store"
	"github.com/task-otter/Taskotter/internal/syncer"
)

type (

	// Result captures sync outcomes for logging, GitHub Actions output, and PR metadata.
	Result struct {
		Plan                 *syncer.Plan
		Ref                  store.RefInfo
		StoreVersion         string
		SourceRef            string
		SourceSHA            string
		TargetFolder         string
		ResolvedTasksJSON    string
		ResolvedDependencies string
		PullRequestNumber    string
		PullRequestURL       string
		Changed              bool
	}

	// ResolvedTask is the JSON representation of a resolved task module mapping.
	ResolvedTask struct {
		SourceModule      string
		DestinationModule string
		Path              string
	}
)

const (
	syncRequiredErrorSuffix = " Merge the sync pull request to update taskfiles, then re-run this workflow.\n"
	jsonIndent              = "  "
)

// MarshalJSON encodes a resolved task using the GitHub Actions output keys.
func (t *ResolvedTask) MarshalJSON() ([]byte, error) {
	//nolint:errchkjson // map[string]string is a safe type; json.Marshal cannot fail
	data, err := json.Marshal(map[string]string{
		"source_module":      t.SourceModule,
		"destination_module": t.DestinationModule,
		"path":               t.Path,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal resolved task: %w", err)
	}

	return data, nil
}

func buildResolvedTasksJSON(requested map[string]syncer.ModuleRecord) (string, error) {
	out := make(map[string]ResolvedTask, len(requested))

	for task := range requested {
		rec := requested[task]

		out[task] = ResolvedTask{
			SourceModule:      rec.SourceModule,
			DestinationModule: rec.DestinationModule,
			Path:              rec.Path,
		}
	}

	data, err := json.MarshalIndent(out, consts.Empty, jsonIndent)
	if err != nil {
		return "", fmt.Errorf("marshal resolved tasks: %w", err)
	}

	return string(data), nil
}

func buildResolvedDependenciesJSON(deps []syncer.ModuleRecord) (string, error) {
	out := make([]ResolvedTask, consts.IndexZero, len(deps))

	for i := range deps {
		dep := &deps[i]

		out = append(out, ResolvedTask{
			SourceModule:      dep.SourceModule,
			DestinationModule: dep.DestinationModule,
			Path:              dep.Path,
		})
	}

	data, err := json.MarshalIndent(out, consts.Empty, jsonIndent)
	if err != nil {
		return "", fmt.Errorf("marshal resolved dependencies: %w", err)
	}

	return string(data), nil
}

func buildResult(cfg *config.Config, plan *syncer.Plan, ref *store.RefInfo) (*Result, error) {
	result := &Result{
		Changed:              plan.Changed,
		StoreVersion:         cfg.StoreVersion,
		SourceRef:            ref.SourceRef,
		SourceSHA:            ref.ResolvedCommit,
		TargetFolder:         cfg.TargetFolder,
		ResolvedTasksJSON:    consts.Empty,
		ResolvedDependencies: consts.Empty,
		PullRequestNumber:    consts.Empty,
		PullRequestURL:       consts.Empty,
		Plan:                 plan,
		Ref:                  *ref,
	}

	var err error

	result.ResolvedTasksJSON, err = buildResolvedTasksJSON(plan.Requested)
	if err != nil {
		return nil, fmt.Errorf("build resolved tasks JSON: %w", err)
	}

	result.ResolvedDependencies, err = buildResolvedDependenciesJSON(plan.Dependencies)
	if err != nil {
		return nil, fmt.Errorf("build resolved dependencies JSON: %w", err)
	}

	return result, nil
}

func printSummary(
	log *logging.Logger,
	cfg *config.Config,
	plan *syncer.Plan,
	result *Result,
	prURL string,
) {
	logRequestedTaskModules(log, cfg, plan)
	logDependencyModules(log, plan)
	logResultMetadata(log, result)
	logFileCounts(log, plan)
	logPullRequestOutcome(log, prURL)
}

func logRequestedTaskModules(log *logging.Logger, cfg *config.Config, plan *syncer.Plan) {
	log.Printf("Requested tasks: %v", cfg.Tasks)

	tasks := cfg.Tasks

	for i := range tasks {
		task := tasks[i]
		rec := plan.Requested[task]
		log.Printf("Source module %s -> %s", rec.SourceModule, rec.Path)
	}
}

func logDependencyModules(log *logging.Logger, plan *syncer.Plan) {
	for i := range plan.Dependencies {
		dep := &plan.Dependencies[i]
		log.Printf("Dependency %s -> %s", dep.SourceModule, dep.Path)
	}
}

func logResultMetadata(log *logging.Logger, result *Result) {
	log.Printf("Store version: %s", empty(result.StoreVersion))
	log.Printf("Source SHA: %s", result.SourceSHA)
	log.Printf(fmtTargetFolder, result.TargetFolder)
}

func logFileCounts(log *logging.Logger, plan *syncer.Plan) {
	log.Printf("Files added: %d", len(plan.Added))
	log.Printf("Files updated: %d", len(plan.Updated))
	log.Printf("Files removed: %d", len(plan.Removed))
}

func logPullRequestOutcome(log *logging.Logger, prURL string) {
	if prURL != consts.Empty {
		log.Printf("Pull request: %s", prURL)
	} else {
		log.Printf("Pull request result: none")
	}
}

func empty(v string) string {
	if v == consts.Empty {
		return "(latest default branch)"
	}

	return v
}

// SyncRequired reports whether the sync run changed managed files.
func SyncRequired(result *Result) bool {
	return result.Changed
}

// ReportSyncRequired writes GitHub Actions annotations when a sync pull request must be merged.
func ReportSyncRequired(result *Result) {
	ReportSyncRequiredTo(os.Stderr, result)
}

// ReportSyncRequiredTo writes sync-required GitHub Actions annotations to writer.
func ReportSyncRequiredTo(writer io.Writer, result *Result) {
	var summary string

	if result.PullRequestURL != consts.Empty {
		prNumber := result.PullRequestNumber

		if prNumber == consts.Empty {
			prNumber = "unknown"
		}

		summary = fmt.Sprintf("TaskOtter opened sync PR #%s: %s", prNumber, result.PullRequestURL)
	} else {
		summary = "TaskOtter synced taskfile changes but did not return a pull request URL."
	}

	//nolint:errcheck // best-effort GitHub Actions annotation write
	_, _ = fmt.Fprintf(
		writer,
		"::error title=TaskOtter sync required::%s"+syncRequiredErrorSuffix,
		summary,
	)
	//nolint:errcheck // best-effort GitHub Actions annotation write
	_, _ = fmt.Fprintln(
		writer,
		"::notice title=What happened::TaskOtter compared managed files with the store and found drift. "+
			"This job fails intentionally until the sync PR is merged.",
	)
}

// ReportSyncUpToDate writes GitHub Actions notices when managed files already match the store.
func ReportSyncUpToDate(result *Result) {
	//nolint:errcheck // best-effort stdout write
	_, _ = fmt.Fprintf(
		os.Stdout,
		"::notice title=TaskOtter sync up to date::Managed taskfiles match the store. No sync pull request was created.\n",
	)
	_, _ = fmt.Fprintf(
		os.Stdout,
		"Store source SHA: %s\n",
		result.SourceSHA,
	)
}

// WriteActionOutputs writes sync result fields to GitHub Actions output or stdout.
func WriteActionOutputs(cfg *config.Config, result *Result) error {
	values := buildOutputValues(result)

	if cfg.GitHubOutput != consts.Empty {
		err := github.WriteOutputs(cfg.GitHubOutput, values)
		if err != nil {
			return fmt.Errorf("write GitHub Actions outputs: %w", err)
		}

		return nil
	}

	printOutputsToStdout(values)

	return nil
}

func buildOutputValues(result *Result) map[string]string {
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

	sort.Strings(keys)

	for i := range keys {
		key := keys[i]

		_, _ = fmt.Fprintf(
			os.Stdout,
			"%s=%s\n",
			key,
			values[key],
		)
	}
}
