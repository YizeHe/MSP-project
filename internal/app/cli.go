package app

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/YizeHe/MSP-project/internal/alert"
)

// RunCLI executes a -c command line (args after -c).
// Returns process exit code.
func RunCLI(dataDir string, args []string) int {
	if len(args) == 0 {
		printCLIHelp()
		return 0
	}
	svc, err := NewService(dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	svc.Log = func(s string) { fmt.Fprintf(os.Stderr, "[msp] %s\n", s) }
	defer svc.WipeSensitive()

	cmd := args[0]
	rest := args[1:]
	var runErr error

	switch cmd {
	case "help", "-h", "--help":
		printCLIHelp()
	case "version", "-v":
		fmt.Println("msp", Version)
	case "init":
		printMN := false
		pass := os.Getenv("MSP_PASSPHRASE")
		for _, a := range rest {
			if a == "--print-mnemonic" || a == "-m" {
				printMN = true
			}
			if strings.HasPrefix(a, "--passphrase=") {
				pass = strings.TrimPrefix(a, "--passphrase=")
			}
		}
		out, err := svc.InitIdentity(printMN, pass)
		runErr = err
		if err == nil {
			printMap(out)
		}
	case "import":
		if len(rest) < 12 {
			runErr = fmt.Errorf("usage: import <bip39 words...>")
			break
		}
		out, err := svc.ImportIdentity(rest, os.Getenv("MSP_PASSPHRASE"))
		runErr = err
		if err == nil {
			printMap(out)
		}
	case "identity", "id":
		out, err := svc.IdentityInfo()
		runErr = err
		if err == nil {
			printMap(out)
		}
	case "seed":
		runErr = cliSeed(svc, rest)
	case "config":
		b, _ := json.MarshalIndent(svc.Cfg, "", "  ")
		fmt.Println(string(b))
	case "health":
		h, err := svc.Health()
		runErr = err
		if err == nil {
			b, _ := json.MarshalIndent(h, "", "  ")
			fmt.Println(string(b))
		}
	case "status":
		b, _ := json.MarshalIndent(svc.Status(), "", "  ")
		fmt.Println(string(b))
	case "start", "join", "run-headless":
		runErr = svc.StartMesh()
		if runErr == nil {
			fmt.Println("mesh started")
			fmt.Printf("node_id=%s udp=%v neighbors=%v\n",
				svc.ID.NodeID, svc.Status()["udp_port"], svc.Status()["p2p_neighbors"])
			if cmd == "run-headless" {
				// keep alive until killed
				select {}
			}
		}
	case "stop":
		svc.StopMesh()
		fmt.Println("mesh stopped")
	case "discover":
		runErr = svc.Discover()
		if runErr == nil {
			fmt.Println("discover ok; neighbors=", svc.WaitPeers(1, 0))
		}
	case "peers":
		punched, disc, err := svc.Peers()
		runErr = err
		if err == nil {
			fmt.Println("p2p_neighbors:")
			for _, p := range punched {
				fmt.Printf("  %s @ %s\n", p["node_id"], p["addr"])
			}
			fmt.Println("seed_discovery:")
			for _, p := range disc {
				fmt.Printf("  %s cands=%v\n", p.NodeID, p.Candidates)
			}
		}
	case "send":
		if len(rest) < 2 {
			runErr = fmt.Errorf("usage: send <node_id> <text...>")
			break
		}
		p, err := svc.Send(rest[0], strings.Join(rest[1:], " "))
		runErr = err
		if err == nil {
			fmt.Println("sent", p.PacketSHA1[:16]+"...", "to", rest[0])
		}
	case "broadcast", "bc":
		if len(rest) < 1 {
			runErr = fmt.Errorf("usage: broadcast <text...>")
			break
		}
		p, err := svc.Broadcast(strings.Join(rest, " "), false)
		runErr = err
		if err == nil {
			fmt.Println("broadcast", p.PacketSHA1[:16]+"...")
		}
	case "alert":
		if len(rest) < 1 {
			runErr = fmt.Errorf("usage: alert <text...>")
			break
		}
		p, err := svc.Broadcast(strings.Join(rest, " "), true)
		runErr = err
		if err == nil {
			fmt.Println("alert-flag broadcast", p.PacketSHA1[:16]+"...")
		}
	case "inbox":
		msgs, err := svc.Inbox()
		runErr = err
		if err == nil {
			if len(msgs) == 0 {
				fmt.Println("(empty)")
			}
			for i, m := range msgs {
				tag := "dm"
				if m.Broadcast {
					tag = "bc"
				}
				if m.Alert {
					tag = "alert"
				}
				fmt.Printf("[%d] %s from=%s via=%s\n    %s\n", i, tag, m.FromID, m.Via, m.Text)
			}
		}
	case "clear-inbox", "clear":
		svc.ClearInbox()
		fmt.Println("inbox wiped")
	case "block":
		if len(rest) < 1 {
			runErr = fmt.Errorf("usage: block <node_id>")
			break
		}
		runErr = svc.Block(rest[0])
		if runErr == nil {
			fmt.Println("blocked", rest[0])
		}
	case "unblock":
		if len(rest) < 1 {
			runErr = fmt.Errorf("usage: unblock <node_id>")
			break
		}
		runErr = svc.Unblock(rest[0])
		if runErr == nil {
			fmt.Println("unblocked", rest[0])
		}
	case "verify-fingerprint", "verify-fp":
		if len(rest) < 2 {
			runErr = fmt.Errorf("usage: verify-fingerprint <node_id> <fp16>")
			break
		}
		runErr = svc.VerifyFingerprint(rest[0], rest[1])
		if runErr == nil {
			fmt.Println("verified", rest[0])
		}
	case "trust":
		for _, r := range svc.TrustList() {
			fmt.Printf("%s verified=%v fp=%s\n", r.NodeID, r.Verified, r.TrustedFP)
		}
	case "mst", "balance":
		if err := svc.EnsureMesh(); err != nil {
			runErr = err
			break
		}
		b, _ := json.MarshalIndent(svc.MSTInfo(), "", "  ")
		fmt.Println(string(b))
	case "claim-genesis", "genesis":
		runErr = svc.ClaimGenesis()
		if runErr == nil {
			fmt.Println("genesis MST claimed")
		}
	case "block-alert":
		runErr = svc.BlockAlert()
		if runErr == nil {
			fmt.Println("blocked official Alert id", alert.NodeID())
		}
	case "alert-info":
		b, _ := json.MarshalIndent(svc.AlertInfo(), "", "  ")
		fmt.Println(string(b))
	case "chain", "chain-status":
		if err := svc.EnsureMesh(); err != nil {
			runErr = err
			break
		}
		b, _ := json.MarshalIndent(svc.ChainStatus(), "", "  ")
		fmt.Println(string(b))
	case "chain-genesis", "genesis-block", "mainnet":
		// frozen mainnet genesis — no node start required
		b, _ := json.MarshalIndent(svc.ChainGenesis(), "", "  ")
		fmt.Println(string(b))
	case "chain-claim":
		runErr = svc.ChainClaimGenesis()
		if runErr == nil {
			fmt.Println("chain genesis_claim mined")
		}
	case "chain-mine":
		force := len(rest) > 0 && rest[0] == "force"
		b, err := svc.ChainMine(force)
		runErr = err
		if err == nil {
			fmt.Printf("mined height=%d hash=%s txs=%d\n", b.Header.Height, b.Header.HashHex()[:16], len(b.Txs))
		}
	case "chain-transfer":
		if len(rest) < 2 {
			runErr = fmt.Errorf("usage: chain-transfer <to> <amount>")
			break
		}
		var amt uint64
		fmt.Sscanf(rest[1], "%d", &amt)
		runErr = svc.ChainTransfer(rest[0], amt)
		if runErr == nil {
			fmt.Println("transfer submitted")
		}
	case "chain-register":
		runErr = svc.ChainRegister()
		if runErr == nil {
			fmt.Println("register tx mined")
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		printCLIHelp()
		return 2
	}

	if runErr != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", runErr)
		return 1
	}
	return 0
}

func cliSeed(svc *Service, rest []string) error {
	if len(rest) == 0 || rest[0] == "list" || rest[0] == "show" {
		for i, s := range svc.SeedList() {
			mark := ""
			if i == 0 {
				mark = " (primary)"
			}
			fmt.Printf("%d. %s%s\n", i+1, s, mark)
		}
		if len(svc.SeedList()) == 0 {
			fmt.Println("(no seeds)")
		}
		return nil
	}
	switch rest[0] {
	case "set":
		if len(rest) < 2 {
			return fmt.Errorf("usage: seed set <url>")
		}
		return svc.SeedSet(rest[1])
	case "add":
		if len(rest) < 2 {
			return fmt.Errorf("usage: seed add <url>")
		}
		return svc.SeedAdd(rest[1])
	case "remove", "rm", "del":
		if len(rest) < 2 {
			return fmt.Errorf("usage: seed remove <url>")
		}
		return svc.SeedRemove(rest[1])
	default:
		return fmt.Errorf("usage: seed list|set|add|remove")
	}
}

func printMap(m map[string]string) {
	for k, v := range m {
		fmt.Printf("%s: %s\n", k, v)
	}
}

func printCLIHelp() {
	fmt.Print(`msp -c <command> [args...]   CLI mode (same capabilities as GUI)

Identity:
  -c init [--print-mnemonic] [--passphrase=...]
  -c import <bip39 words...>
  -c identity

Seeds (signaling / hole-punch):
  -c seed list|set <url>|add <url>|remove <url>
  -c health

Mesh / P2P:
  -c start | stop | status | discover | peers
  -c send <node_id> <text...>
  -c broadcast <text...>
  -c alert <text...>
  -c inbox | clear-inbox
  -c block|unblock <node_id>
  -c verify-fingerprint <node_id> <fp16>
  -c trust
  -c mst | claim-genesis          (local DTN wallet: 128 MST, one-time; optional)
  -c block-alert | alert-info

Public chain / mainnet (economy only — no message content on-chain):
  -c chain-genesis | mainnet     frozen genesis (msp-mainnet-1), no start needed
  -c chain | chain-status
  -c chain-claim | chain-mine [force]
  -c chain-transfer <to> <amount>
  -c chain-register
  send/broadcast: one-time burner + BurnTicket (RefHash=cipher) + DTN payload.
  Mainnet: claim 128 MST × 210 nodes; miner pool 4.2M; total 4,226,880.

Env:
  MSP_DATA              data directory (default ./msp-data)
  MSP_PASSPHRASE        optional identity encryption passphrase
  MSP_POW_FAST=1        fast PoW targets (~2s) for lab
  MSP_CHAIN_FAST=1      short block interval for lab
  MSP_REQUIRE_TICKET=0  disable receive-side BurnTicket enforcement (lab)
  SOCKS5_PROXY          socks5://127.0.0.1:9050 for Tor onion seeds
  ALL_PROXY             same

Without -c, msp launches the GUI.
`)
}
