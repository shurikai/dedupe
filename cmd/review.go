package cmd

import (
	"context"
	"dedupe/server"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	reviewPort   int
	reviewOutput string
)

var reviewCmd = &cobra.Command{
	Use:   "review <pairs-file>",
	Short: "Start a local web server to visually review duplicate pairs",
	Args:  cobra.ExactArgs(1),
	RunE:  runReview,
}

func init() {
	rootCmd.AddCommand(reviewCmd)
	reviewCmd.Flags().IntVar(&reviewPort, "port", 8080, "Port to listen on")
	reviewCmd.Flags().StringVar(&reviewOutput, "output", "", "Path for decisions output file (default: <pairs-file>.decisions.json)")
}

func runReview(_ *cobra.Command, args []string) error {
	pairsFile := args[0]

	pairs, err := server.ParsePairFile(pairsFile)
	if err != nil {
		return fmt.Errorf("parsing pair file: %w", err)
	}

	decisionsPath := reviewOutput
	if decisionsPath == "" {
		ext := filepath.Ext(pairsFile)
		decisionsPath = strings.TrimSuffix(pairsFile, ext) + ".decisions.json"
	}

	if err := server.LoadDecisions(pairs, decisionsPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("loading existing decisions: %w", err)
	}

	srv := server.New(pairs, decisionsPath)

	url := fmt.Sprintf("http://localhost:%d", reviewPort)
	fmt.Printf("Starting review server at %s\n", url)
	fmt.Printf("Pair file:   %s (%d pairs)\n", pairsFile, len(pairs))
	fmt.Printf("Decisions:   %s\n", decisionsPath)
	fmt.Println("Press Ctrl+C to stop.")

	openBrowser(url)

	httpSrv := &http.Server{
		Addr:    fmt.Sprintf(":%d", reviewPort),
		Handler: srv,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		fmt.Println("\nShutting down...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return httpSrv.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		fmt.Printf("Could not open browser automatically. Visit: %s\n", url)
	}
}
