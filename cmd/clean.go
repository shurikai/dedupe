package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var (
	cleanExecute bool
	cleanDryRun  bool
	cleanMoveTo  string
	cleanYes     bool
)

var cleanCmd = &cobra.Command{
	Use:   "clean <decisions-file>",
	Short: "Execute recorded decisions: delete or move the marked files",
	Args:  cobra.ExactArgs(1),
	RunE:  runClean,
}

func init() {
	rootCmd.AddCommand(cleanCmd)
	cleanCmd.Flags().BoolVar(&cleanExecute, "execute", false, "Actually perform file operations (default is dry-run)")
	cleanCmd.Flags().BoolVar(&cleanDryRun, "dry-run", false, "Print planned actions without executing (default behavior)")
	cleanCmd.Flags().StringVar(&cleanMoveTo, "move-to", "", "Move marked files to this directory instead of deleting")
	cleanCmd.Flags().BoolVar(&cleanYes, "yes", false, "Skip confirmation prompt")
}

type decisionEntry struct {
	Left     string `json:"left"`
	Right    string `json:"right"`
	Decision string `json:"decision"`
}

func runClean(_ *cobra.Command, args []string) error {
	entries, err := readDecisionsFile(args[0])
	if err != nil {
		return fmt.Errorf("reading decisions file: %w", err)
	}

	type planned struct{ path string }
	var toAct []planned
	var kept, unsureCount int
	var unsurePairs []decisionEntry

	for _, e := range entries {
		switch e.Decision {
		case "delete_left":
			toAct = append(toAct, planned{path: e.Left})
		case "delete_right":
			toAct = append(toAct, planned{path: e.Right})
		case "keep_both":
			kept++
		default: // "unsure" or ""
			unsureCount++
			unsurePairs = append(unsurePairs, e)
		}
	}

	// Summary line (matches CLAUDE.md spec)
	action := "deleted"
	actionInf := "delete"
	if cleanMoveTo != "" {
		action = "moved to " + cleanMoveTo
		actionInf = "move to " + cleanMoveTo
	}
	fmt.Printf("%d file(s) will be %s. %d pair(s) kept. %d unsure.\n",
		len(toAct), action, kept, unsureCount)

	if unsureCount > 0 {
		fmt.Println("\nUnsure / unreviewed pairs (no action taken):")
		for _, e := range unsurePairs {
			fmt.Printf("  %s\n  %s\n\n", e.Left, e.Right)
		}
	}

	if len(toAct) == 0 {
		fmt.Println("Nothing to do.")
		return nil
	}

	fmt.Printf("\nFiles to %s:\n", actionInf)
	for _, a := range toAct {
		fmt.Printf("  %s\n", a.path)
	}
	fmt.Println()

	// --dry-run is the default; --execute is required to act. --dry-run wins if both are passed.
	isDryRun := !cleanExecute || cleanDryRun
	if isDryRun {
		fmt.Println("Dry-run: no changes made. Pass --execute to apply.")
		return nil
	}

	if !cleanYes {
		if !confirmPrompt(fmt.Sprintf("Proceed? This will %s %d file(s). [y/N] ", actionInf, len(toAct))) {
			fmt.Println("Aborted.")
			return nil
		}
	}

	if cleanMoveTo != "" {
		if err := os.MkdirAll(cleanMoveTo, 0755); err != nil {
			return fmt.Errorf("creating move-to directory %s: %w", cleanMoveTo, err)
		}
	}

	var failed int
	for _, a := range toAct {
		if cleanMoveTo != "" {
			dest := filepath.Join(cleanMoveTo, filepath.Base(a.path))
			if err := renameOrCopy(a.path, dest); err != nil {
				fmt.Fprintf(os.Stderr, "error: move %s: %s\n", a.path, err)
				failed++
			} else {
				fmt.Printf("moved:   %s\n", a.path)
			}
		} else {
			if err := os.Remove(a.path); err != nil {
				fmt.Fprintf(os.Stderr, "error: delete %s: %s\n", a.path, err)
				failed++
			} else {
				fmt.Printf("deleted: %s\n", a.path)
			}
		}
	}

	if failed > 0 {
		return fmt.Errorf("%d of %d operation(s) failed", failed, len(toAct))
	}

	fmt.Printf("\nDone. %d file(s) processed.\n", len(toAct))
	return nil
}

func readDecisionsFile(path string) ([]decisionEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var entries []decisionEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parsing JSON: %w", err)
	}
	return entries, nil
}

func confirmPrompt(prompt string) bool {
	fmt.Print(prompt)
	r := bufio.NewReader(os.Stdin)
	line, _ := r.ReadString('\n')
	return strings.TrimSpace(strings.ToLower(line)) == "y"
}

// renameOrCopy tries os.Rename first; falls back to copy+delete for cross-device moves.
func renameOrCopy(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	if err := copyFileForMove(src, dst); err != nil {
		return err
	}
	return os.Remove(src)
}

func copyFileForMove(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
