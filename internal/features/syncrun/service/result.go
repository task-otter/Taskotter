// Taskotter 2026.
// SPDX-License-Identifier: Apache-2.0.

package service

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"

	storedomain "github.com/task-otter/Taskotter/internal/features/store/domain"
	syncdomain "github.com/task-otter/Taskotter/internal/features/sync/domain"
	"github.com/task-otter/Taskotter/internal/features/sync/domain/lockmodel"
	"github.com/task-otter/Taskotter/internal/shared/config"
	"github.com/task-otter/Taskotter/internal/shared/consts"
	"github.com/task-otter/Taskotter/internal/shared/iox"
	"github.com/task-otter/Taskotter/internal/shared/logging"
)

type (

	// Result captures sync outcomes for logging, GitHub Actions output, and PR metadata.
	Result struct {
		Plan                 *syncdomain.Plan
		Ref                  storedomain.RefInfo
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

	summaryInput struct {
		Log    *logging.Logger
		Cfg    *config.Config
		Plan   *syncdomain.Plan
		Result *Result
		PRURL  string
	}
)

const (
	syncRequiredErrorSuffix = " Merge the sync pull request to update taskfiles, then re-run this workflow.\n"

	jsonIndent = "  "

	syncRequiredNotice = "::notice title=What happened::TaskOtter compared managed files " +
		"with the store and found drift. This job fails intentionally until the sync PR is merged.\n"

	syncUpToDateNotice = "::notice title=TaskOtter sync up to date::Managed taskfiles " +
		"match the store. No sync pull request was created.\n"
)

// ReportSyncRequired writes GitHub Actions annotations when a sync pull request must be merged.
func ReportSyncRequired(result *Result) {
	ReportSyncRequiredTo(os.Stderr, result)
}

// ReportSyncRequiredTo writes sync-required GitHub Actions annotations to writer.
func ReportSyncRequiredTo(writer io.Writer, result *Result) {
	writeSyncRequiredAnnotations(writer, syncRequiredSummary(result))
}

// ReportSyncUpToDate writes GitHub Actions notices when managed files already match the store.
func ReportSyncUpToDate(result *Result) {
	iox.FprintBestEffort(os.Stdout, syncUpToDateNotice)
	iox.FprintfBestEffortf(os.Stdout, "Store source SHA: %s\n", result.SourceSHA)
}

// SyncRequired reports whether the sync run changed managed files.
func SyncRequired(result *Result) bool {
	return result.Changed
}

// WriteActionOutputs writes sync result fields to GitHub Actions output or stdout.
func WriteActionOutputs(cfg *config.Config, result *Result) error {
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

func buildResolvedDependenciesJSON(deps []lockmodel.ModuleRecord) string {
	out := make([]ResolvedTask, consts.IndexZero, len(deps))

	for i := range deps {
		dep := &deps[i]

		out = append(out, ResolvedTask{
			SourceModule:      dep.SourceModule,
			DestinationModule: dep.DestinationModule,
			Path:              dep.Path,
		})
	}

	//nolint:dogsled,errcheck,gosec // errchkjson: ResolvedTask slice is safe to marshal
	data, _ := json.MarshalIndent(out, consts.Empty, jsonIndent)

	return string(data)
}

func buildResolvedTasksJSON(requested map[string]lockmodel.ModuleRecord) string {
	out := make(map[string]ResolvedTask, len(requested))

	for task := range requested {
		rec := requested[task]

		out[task] = ResolvedTask{
			SourceModule:      rec.SourceModule,
			DestinationModule: rec.DestinationModule,
			Path:              rec.Path,
		}
	}

	//nolint:dogsled,errcheck,gosec // errchkjson: map[string]ResolvedTask is safe to marshal
	data, _ := json.MarshalIndent(out, consts.Empty, jsonIndent)

	return string(data)
}

func buildResult(cfg *config.Config, plan *syncdomain.Plan, ref *storedomain.RefInfo) *Result {
	result := newResultShell(cfg, plan, ref)
	fillResolvedJSON(result, plan)

	return result
}

func empty(v string) string {
	if v == consts.Empty {
		return "(latest default branch)"
	}

	return v
}

func fillResolvedJSON(result *Result, plan *syncdomain.Plan) {
	result.ResolvedTasksJSON = buildResolvedTasksJSON(plan.Requested)
	result.ResolvedDependencies = buildResolvedDependenciesJSON(plan.Dependencies)
}

func logDependencyModules(log *logging.Logger, plan *syncdomain.Plan) {
	for i := range plan.Dependencies {
		dep := &plan.Dependencies[i]
		log.Printf("Dependency %s -> %s", dep.SourceModule, dep.Path)
	}
}

func logFileCounts(log *logging.Logger, plan *syncdomain.Plan) {
	log.Printf("Files added: %d", len(plan.Added))
	log.Printf("Files updated: %d", len(plan.Updated))
	log.Printf("Files removed: %d", len(plan.Removed))
}

func logPullRequestOutcome(log *logging.Logger, prURL string) {
	if prURL != consts.Empty {
		log.Printf("Pull request: %s", prURL)

		return
	}

	log.Print("Pull request result: none")
}

func logRequestedTaskModules(log *logging.Logger, cfg *config.Config, plan *syncdomain.Plan) {
	log.Printf("Requested tasks: %v", cfg.Tasks)

	tasks := cfg.Tasks

	for i := range tasks {
		task := tasks[i]
		rec := plan.Requested[task]
		log.Printf("Source module %s -> %s", rec.SourceModule, rec.Path)
	}
}

func logResultMetadata(log *logging.Logger, result *Result) {
	log.Printf("Store version: %s", empty(result.StoreVersion))
	log.Printf("Source SHA: %s", result.SourceSHA)
	log.Printf(fmtTargetFolder, result.TargetFolder)
}

func newResultShell(cfg *config.Config, plan *syncdomain.Plan, ref *storedomain.RefInfo) *Result {
	return &Result{
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
}

func printOutputsToStdout(values map[string]string) {
	keys := make([]string, consts.IndexZero, len(values))

	for key := range values {
		keys = append(keys, key)
	}

	slices.Sort(keys)

	for i := range keys {
		key := keys[i]

		iox.FprintfBestEffortf(
			os.Stdout,
			"%s=%s\n",
			key,
			values[key],
		)
	}
}

func printSummary(in *summaryInput) {
	logRequestedTaskModules(in.Log, in.Cfg, in.Plan)
	logDependencyModules(in.Log, in.Plan)
	logResultMetadata(in.Log, in.Result)
	logFileCounts(in.Log, in.Plan)
	logPullRequestOutcome(in.Log, in.PRURL)
}

func syncRequiredSummary(result *Result) string {
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
