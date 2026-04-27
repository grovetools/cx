// Package view hosts the `cx view` CLI command. The actual TUI model
// lives in github.com/grovetools/cx/pkg/tui/view so it can be embedded
// by other hosts (e.g. grove-terminal). This file is a thin wrapper
// that constructs the model, runs it through bubbletea, and forwards
// the Neovim IPC exit marker if the model asked to quit for editing.
package view

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/grovetools/compositor"
	"github.com/grovetools/core/config"
	"github.com/spf13/cobra"

	tuiView "github.com/grovetools/cx/pkg/tui/view"
)

// NewViewCmd creates the view command.
func NewViewCmd() *cobra.Command {
	var startPage string
	cmd := &cobra.Command{
		Use:   "view",
		Short: "Display an interactive visualization of context composition",
		Long:  `Launch an interactive terminal UI that shows which files are included, excluded, or ignored in your context based on rules and git ignore patterns.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			workDir, _ := cmd.Flags().GetString("dir")
			cfg, _ := config.LoadFrom(".")
			m, err := tuiView.NewWithStartPage(startPage, workDir, "", cfg)
			if err != nil {
				return err
			}
			compModel := compositor.NewModel(m)
			p := tea.NewProgram(compModel, tea.WithAltScreen())
			finalModel, err := p.Run()
			if err != nil {
				return err
			}

			// Close the model to release file-watcher resources.
			m.Close()

			// Free compositor resources and unwrap to recover the tuiView.Model
			// so post-exit type assertions succeed.
			if cm, ok := finalModel.(*compositor.Model); ok {
				cm.Free()
				finalModel = cm.Unwrap()
			}

			// After the TUI exits, check if it was for Neovim editing integration.
			if vm, ok := finalModel.(tuiView.Model); ok && vm.ExitForNvimEdit {
				// Print the special string for the Neovim plugin to capture.
				fmt.Println("EDIT_FILE:" + vm.NvimEditPath)
			}

			return nil
		},
	}
	cmd.Flags().StringVarP(&startPage, "page", "p", "tree", "The page to open on startup (tree, rules, stats, list)")
	return cmd
}
