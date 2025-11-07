package cmd

import (
	"fmt"
	"sort"
	"github.com/spf13/cobra"
	"github.com/mattsolo1/grove-context/pkg/context"
)

func NewDiffCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diff [ruleset-name]",
		Short: "Compare the current context with a named rule set",
		Long:  `Compare the current context with a named rule set from .cx/ or .cx.work/ to see added/removed files, token count changes, and size differences.`,
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := context.NewManager("")
			
			compareName := "empty"
			if len(args) > 0 {
				compareName = args[0]
			}
			
			diff, err := mgr.DiffContext(compareName)
			if err != nil {
				return err
			}
			
			printDiff(diff, compareName)
			return nil
		},
	}
	
	return cmd
}

// printDiff displays the diff result using the pretty logger
func printDiff(d *context.DiffResult, compareName string) {
	prettyLog.InfoPretty(fmt.Sprintf("Comparing current context with '%s':", compareName))
	prettyLog.Blank()
	
	// Show added files
	if len(d.Added) > 0 {
		prettyLog.InfoPretty(fmt.Sprintf("Added files (%d):", len(d.Added)))
		sort.Slice(d.Added, func(i, j int) bool {
			return d.Added[i].Tokens > d.Added[j].Tokens
		})
		for _, f := range d.Added {
			line := fmt.Sprintf("%-50s (%s tokens)", context.TruncatePath(f.Path, 50), context.FormatTokenCount(f.Tokens))
			prettyLog.Success("  + " + line)
		}
		prettyLog.Blank()
	}
	
	// Show removed files
	if len(d.Removed) > 0 {
		prettyLog.InfoPretty(fmt.Sprintf("Removed files (%d):", len(d.Removed)))
		sort.Slice(d.Removed, func(i, j int) bool {
			return d.Removed[i].Tokens > d.Removed[j].Tokens
		})
		for _, f := range d.Removed {
			line := fmt.Sprintf("%-50s (%s tokens)", context.TruncatePath(f.Path, 50), context.FormatTokenCount(f.Tokens))
			prettyLog.ErrorPretty("  - " + line, nil)
		}
		prettyLog.Blank()
	}
	
	// Show summary
	prettyLog.InfoPretty("Summary:")
	fileDiff := len(d.CurrentFiles) - len(d.CompareFiles)
	fileSign := ""
	if fileDiff > 0 {
		fileSign = "+"
	}
	prettyLog.InfoPretty(fmt.Sprintf("  Files: %d → %d (%s%d)", 
		len(d.CompareFiles), len(d.CurrentFiles), fileSign, fileDiff))
	
	tokenDiff := d.CurrentTotalTokens - d.CompareTotalTokens
	tokenSign := ""
	if tokenDiff > 0 {
		tokenSign = "+"
	}
	prettyLog.InfoPretty(fmt.Sprintf("  Tokens: %s → %s (%s%s)",
		context.FormatTokenCount(d.CompareTotalTokens),
		context.FormatTokenCount(d.CurrentTotalTokens),
		tokenSign,
		context.FormatTokenCount(abs(tokenDiff))))
	
	sizeDiff := d.CurrentTotalSize - d.CompareTotalSize
	sizeSign := ""
	if sizeDiff > 0 {
		sizeSign = "+"
	}
	prettyLog.InfoPretty(fmt.Sprintf("  Size: %s → %s (%s%s)",
		context.FormatBytes(int(d.CompareTotalSize)),
		context.FormatBytes(int(d.CurrentTotalSize)),
		sizeSign,
		context.FormatBytes(int(abs64(sizeDiff)))))
}

// abs returns the absolute value of an integer
func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// abs64 returns the absolute value of an int64
func abs64(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}