package cmd

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"

	"github.com/grovetools/cx/pkg/context"
	"github.com/spf13/cobra"
)

const machineSchemaVersion = 1

func writeJSON(cmd *cobra.Command, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal machine output: %w", err)
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return nil
}

// machineFileSet is one resolved hot or cold context. ResolvedFiles counts
// every path produced by the rules engine; ReadableFiles counts only paths the
// stats provider could inspect. Manifests and largest-file lists are bounded.
type machineFileSet struct {
	ContextType            string                            `json:"context_type"`
	ResolvedFiles          int                               `json:"resolved_files"`
	ReadableFiles          int                               `json:"readable_files"`
	TotalTokens            int                               `json:"total_tokens"`
	TotalSize              int64                             `json:"total_size"`
	Languages              map[string]*context.LanguageStats `json:"languages"`
	LargestFiles           []context.FileStats               `json:"largest_files"`
	LargestFilesOmitted    int                               `json:"largest_files_omitted"`
	Files                  []string                          `json:"files"`
	FilesOmitted           int                               `json:"files_omitted"`
	UnreadableFiles        []string                          `json:"unreadable_files"`
	UnreadableFilesOmitted int                               `json:"unreadable_files_omitted"`
}

type machineTotals struct {
	ResolvedFiles int   `json:"resolved_files"`
	ReadableFiles int   `json:"readable_files"`
	TotalTokens   int   `json:"total_tokens"`
	TotalSize     int64 `json:"total_size"`
	Unreadable    int   `json:"unreadable_files"`
}

type machineStatsEnvelope struct {
	SchemaVersion int              `json:"schema_version"`
	Workspace     string           `json:"workspace,omitempty"`
	RulesPath     string           `json:"rules_path"`
	Contexts      []machineFileSet `json:"contexts"`
	Totals        machineTotals    `json:"totals"`
}

type machineListTotals struct {
	Hot   int `json:"hot_files"`
	Cold  int `json:"cold_files"`
	Total int `json:"total_files"`
}

type machineListEnvelope struct {
	SchemaVersion int               `json:"schema_version"`
	Workspace     string            `json:"workspace,omitempty"`
	RulesPath     string            `json:"rules_path"`
	HotFiles      []string          `json:"hot_files"`
	ColdFiles     []string          `json:"cold_files"`
	Totals        machineListTotals `json:"totals"`
}

func resolveMachineFiles(mgr *context.Manager, targetRulesFile string) (hotFiles, coldFiles []string, rulesPath string, err error) {
	if targetRulesFile != "" {
		hotFiles, coldFiles, err = mgr.ResolveFilesFromCustomRulesFile(targetRulesFile)
		if err != nil {
			return nil, nil, "", fmt.Errorf("failed to resolve files from custom rules file: %w", err)
		}
		rulesPath = targetRulesFile
		if !filepath.IsAbs(rulesPath) {
			rulesPath = filepath.Join(mgr.GetWorkDir(), rulesPath)
		}
		rulesPath, _ = filepath.Abs(rulesPath)
		return hotFiles, coldFiles, rulesPath, nil
	}

	hotFiles, err = mgr.ResolveFilesFromRules()
	if err != nil {
		return nil, nil, "", err
	}
	coldFiles, err = mgr.ResolveColdContextFiles()
	if err != nil {
		return nil, nil, "", err
	}
	rulesPath = mgr.ResolveRulesPath()
	return hotFiles, coldFiles, rulesPath, nil
}

func absoluteMachinePaths(files []string, base string) []string {
	paths := make([]string, 0, len(files))
	for _, file := range files {
		paths = append(paths, projectListPath(file, base, false))
	}
	sort.Strings(paths)
	if paths == nil {
		return []string{}
	}
	return paths
}

func boundedStrings(items []string, limit int) ([]string, int) {
	if limit < 0 {
		limit = 0
	}
	if limit > len(items) {
		limit = len(items)
	}
	out := append([]string(nil), items[:limit]...)
	if out == nil {
		out = []string{}
	}
	return out, len(items) - limit
}

func buildMachineFileSet(mgr *context.Manager, contextType string, files []string, top, manifestLimit int) (machineFileSet, error) {
	stats, err := mgr.GetStats(contextType, files, top)
	if err != nil {
		return machineFileSet{}, err
	}

	resolved := absoluteMachinePaths(files, mgr.GetWorkDir())
	readable := make(map[string]bool, len(stats.AllFiles))
	for _, file := range stats.AllFiles {
		readable[projectListPath(file.Path, mgr.GetWorkDir(), false)] = true
	}
	unreadable := make([]string, 0)
	for _, file := range resolved {
		if !readable[file] {
			unreadable = append(unreadable, file)
		}
	}

	manifest, manifestOmitted := boundedStrings(resolved, manifestLimit)
	unreadableManifest, unreadableOmitted := boundedStrings(unreadable, manifestLimit)
	largest := append([]context.FileStats(nil), stats.LargestFiles...)
	if largest == nil {
		largest = []context.FileStats{}
	}
	languages := stats.Languages
	if languages == nil {
		languages = map[string]*context.LanguageStats{}
	}

	return machineFileSet{
		ContextType:            contextType,
		ResolvedFiles:          len(resolved),
		ReadableFiles:          len(stats.AllFiles),
		TotalTokens:            stats.TotalTokens,
		TotalSize:              stats.TotalSize,
		Languages:              languages,
		LargestFiles:           largest,
		LargestFilesOmitted:    len(stats.AllFiles) - len(largest),
		Files:                  manifest,
		FilesOmitted:           manifestOmitted,
		UnreadableFiles:        unreadableManifest,
		UnreadableFilesOmitted: unreadableOmitted,
	}, nil
}

func buildMachineStats(mgr *context.Manager, workspaceName, rulesPath string, hotFiles, coldFiles []string, top, manifestLimit int) (machineStatsEnvelope, error) {
	envelope := machineStatsEnvelope{
		SchemaVersion: machineSchemaVersion,
		Workspace:     workspaceName,
		RulesPath:     rulesPath,
		Contexts:      []machineFileSet{},
	}
	for _, item := range []struct {
		name  string
		files []string
	}{{"hot", hotFiles}, {"cold", coldFiles}} {
		set, err := buildMachineFileSet(mgr, item.name, item.files, top, manifestLimit)
		if err != nil {
			return machineStatsEnvelope{}, fmt.Errorf("failed to compute %s stats: %w", item.name, err)
		}
		envelope.Contexts = append(envelope.Contexts, set)
		envelope.Totals.ResolvedFiles += set.ResolvedFiles
		envelope.Totals.ReadableFiles += set.ReadableFiles
		envelope.Totals.TotalTokens += set.TotalTokens
		envelope.Totals.TotalSize += set.TotalSize
		envelope.Totals.Unreadable += set.ResolvedFiles - set.ReadableFiles
	}
	return envelope, nil
}

func buildMachineList(mgr *context.Manager, workspaceName, rulesPath string, hotFiles, coldFiles []string) machineListEnvelope {
	hot := absoluteMachinePaths(hotFiles, mgr.GetRulesBaseDir())
	cold := absoluteMachinePaths(coldFiles, mgr.GetRulesBaseDir())
	return machineListEnvelope{
		SchemaVersion: machineSchemaVersion,
		Workspace:     workspaceName,
		RulesPath:     rulesPath,
		HotFiles:      hot,
		ColdFiles:     cold,
		Totals: machineListTotals{
			Hot:   len(hot),
			Cold:  len(cold),
			Total: len(hot) + len(cold),
		},
	}
}
