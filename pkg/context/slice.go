package context

import (
	"bufio"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

// A "slice" is a rules file that never touches disk: a caller hands cx a few
// lines, cx resolves them through the ordinary rules engine and renders the
// matched files to a stream. It exists so an agent holding a curated bundle can
// widen its own context by naming a pattern, without minting a rules file, a
// .grove/context, or any other artifact the next `cx generate` would have to
// reconcile. Everything about resolution — aliases, globs, brace expansion,
// directives, junk-dir skipping, workspace sandboxing — is the shared engine's;
// only the input source and the output target are new.

const (
	// sliceRulesName is the synthetic rules-file name slice lines expand
	// under. Nothing is written there; it exists to give @include: and
	// @default: lines a directory to resolve against, and it sits directly
	// under workDir so that directory is the workspace root — the same base
	// a root-level rules file would give them.
	sliceRulesName = "<cx-slice>.rules"

	// sliceIndent matches the two-space indentation flow's layer renderer
	// uses for its <file> blocks, so a slice and a layer carrying the same
	// file are byte-identical.
	sliceIndent = "  "
)

// ResolveSlice resolves ad-hoc rules lines into a file set, exactly as those
// lines would resolve inside a rules file at the workspace root. Returned paths
// are relative to the rules base dir where possible (absolute otherwise) and
// sorted, matching ResolveFilesFromCustomRulesFile.
//
// treePaths carries any @tree:/@t: paths the lines asked for. Slice renders
// files only, so the caller is responsible for telling the user those lines
// produced nothing.
//
// A '---' separator is rejected rather than honored: slice emits one flat
// stream, so silently dropping everything below the separator would thin the
// caller's context without saying so.
func (m *Manager) ResolveSlice(lines []string) (files, treePaths []string, err error) {
	if len(lines) == 0 {
		return nil, nil, fmt.Errorf("no slice lines given")
	}

	content := []byte(strings.Join(lines, "\n") + "\n")
	syntheticPath := filepath.Join(m.workDir, sliceRulesName)

	hotRules, coldRules, _, treePaths, err := m.expandRulesContent(
		syntheticPath, content, map[string]bool{syntheticPath: true}, 0, true)
	if err != nil {
		return nil, nil, err
	}
	if len(coldRules) > 0 {
		return nil, nil, fmt.Errorf("cx slice resolves one flat file set; the '---' hot/cold separator is not supported (%d line(s) fell below it)", len(coldRules))
	}

	files, err = m.resolveFilesViaAST(hotRules)
	if err != nil {
		return nil, nil, err
	}
	return files, deduplicateStrings(treePaths), nil
}

// WriteSlice renders files as the standard <file path=…> blocks — the same
// bytes generate and flow's layer renderer emit — inside a <slice> envelope so
// the stream is a single well-formed XML document. Honors SetStripComments.
//
// An unreadable file leaves generate's inline <error> placeholder in the stream
// AND returns an error, so a caller that ignores the exit code still sees the
// hole and one that checks it never mistakes a thinned bundle for a whole one.
func (m *Manager) WriteSlice(w io.Writer, files []string) error {
	bw := bufio.NewWriter(w)
	fmt.Fprintf(bw, "<slice files=\"%d\">\n", len(files))

	var firstErr error
	unreadable := 0
	for _, file := range files {
		if err := m.writeFileToXML(bw, file, sliceIndent); err != nil {
			unreadable++
			if firstErr == nil {
				firstErr = fmt.Errorf("reading context file %s: %w", file, err)
			}
		}
	}

	bw.WriteString("</slice>\n")
	if err := bw.Flush(); err != nil {
		return err
	}
	if firstErr != nil {
		return fmt.Errorf("%d of %d slice file(s) unreadable; first: %w", unreadable, len(files), firstErr)
	}
	return nil
}
