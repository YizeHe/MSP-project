// Command msp — GUI by default; full CLI via -c / --cli.
package main

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/YizeHe/MSP-project/internal/app"
	"github.com/YizeHe/MSP-project/internal/config"
	"github.com/YizeHe/MSP-project/internal/gui"
)

func main() {
	dataDir := envOr("MSP_DATA", config.DefaultDir())
	args := os.Args[1:]

	if len(args) == 0 {
		runGUI(dataDir)
		return
	}

	// Parse global flags / mode
	var (
		cliMode  bool
		cliArgs  []string
		forceGUI bool
	)
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-h" || a == "--help":
			printRootHelp()
			return
		case a == "-v" || a == "--version":
			fmt.Println("msp", app.Version)
			return
		case a == "--gui":
			forceGUI = true
		case a == "-c" || a == "--cli" || a == "/c":
			cliMode = true
			cliArgs = args[i+1:]
			i = len(args)
		case strings.HasPrefix(a, "--data="):
			dataDir = strings.TrimPrefix(a, "--data=")
		case a == "--data" && i+1 < len(args):
			dataDir = args[i+1]
			i++
		default:
			// Convenience: bare commands without -c still work as CLI
			if !cliMode && !forceGUI {
				cliMode = true
				cliArgs = args[i:]
				i = len(args)
			}
		}
	}

	if forceGUI || (!cliMode && len(cliArgs) == 0) {
		runGUI(dataDir)
		return
	}
	os.Exit(app.RunCLI(dataDir, cliArgs))
}

func runGUI(dataDir string) {
	// Allow Ctrl+C
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	errCh := make(chan error, 1)
	go func() { errCh <- gui.Run(dataDir) }()
	select {
	case <-sig:
		fmt.Println("\nbye")
		os.Exit(0)
	case err := <-errCh:
		if err != nil {
			fmt.Fprintf(os.Stderr, "gui error: %v\n", err)
			fmt.Fprintln(os.Stderr, "Tip: msp -c help")
			os.Exit(1)
		}
	}
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func printRootHelp() {
	fmt.Print(`msp — MST Network client v0.3

  msp                      Launch GUI (local browser UI)
  msp -c <command> ...     Full CLI (all GUI actions)
  msp --gui                Force GUI
  msp --data <dir>         Data directory (or MSP_DATA)

Examples:
  msp -c init --print-mnemonic
  msp -c seed set https://msp.forbiddenx.top
  msp -c start
  msp -c peers
  msp -c send <node_id> hello
  msp -c inbox
  msp -c verify-fingerprint <node_id> <fp16>
  msp -c help

Architecture:
  Cloudflare seed = signaling / hole-punch ONLY
  Messages = UDP P2P mesh (not via CF)

Env: MSP_DATA, MSP_PASSPHRASE, HTTP_PROXY (signaling only)
`)
}
