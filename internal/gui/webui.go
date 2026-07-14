// Package gui provides a local web desktop UI (opens system browser).
// All actions go through app.Service — same as msp -c.
package gui

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/YizeHe/MSP-project/internal/app"
)

//go:embed index.html app.js
var staticFS embed.FS

// Run starts HTTP UI, opens browser, blocks until ctx cancelled via returned server.
func Run(dataDir string) error {
	svc, err := app.NewService(dataDir)
	if err != nil {
		return err
	}
	svc.Log = func(s string) { fmt.Printf("[msp] %s\n", s) }
	defer svc.WipeSensitive()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	url := "http://" + ln.Addr().String() + "/"

	mux := http.NewServeMux()
	sub, _ := fs.Sub(staticFS, ".")
	fileServer := http.FileServer(http.FS(sub))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			b, _ := staticFS.ReadFile("index.html")
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(b)
			return
		}
		fileServer.ServeHTTP(w, r)
	})
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		handleAPI(w, r, svc)
	})

	srv := &http.Server{Handler: mux}
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(ln) }()
	time.Sleep(150 * time.Millisecond)
	_ = openBrowser(url)
	fmt.Println("MSP GUI:", url)
	fmt.Println("CLI equivalent: msp -c <command>")
	fmt.Println("Press Ctrl+C to quit.")

	err = <-errCh
	_ = srv.Shutdown(context.Background())
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

func handleAPI(w http.ResponseWriter, r *http.Request, svc *app.Service) {
	w.Header().Set("Content-Type", "application/json")
	path := strings.TrimPrefix(r.URL.Path, "/api/")
	writeErr := func(err error) {
		w.WriteHeader(400)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
	}
	writeOK := func(v any) { _ = json.NewEncoder(w).Encode(v) }

	switch {
	case path == "status" && r.Method == http.MethodGet:
		writeOK(svc.Status())
	case path == "identity" && r.Method == http.MethodGet:
		out, err := svc.IdentityInfo()
		if err != nil {
			writeErr(err)
			return
		}
		writeOK(out)
	case path == "init" && r.Method == http.MethodPost:
		var body struct {
			PrintMnemonic bool   `json:"print_mnemonic"`
			Passphrase    string `json:"passphrase"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		out, err := svc.InitIdentity(body.PrintMnemonic, body.Passphrase)
		if err != nil {
			writeErr(err)
			return
		}
		writeOK(out)
	case path == "seeds" && r.Method == http.MethodGet:
		writeOK(map[string]any{"seeds": svc.SeedList()})
	case path == "seed/set" && r.Method == http.MethodPost:
		var body struct {
			URL string `json:"url"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if err := svc.SeedSet(body.URL); err != nil {
			writeErr(err)
			return
		}
		writeOK(map[string]string{"ok": "1"})
	case path == "seed/add" && r.Method == http.MethodPost:
		var body struct {
			URL string `json:"url"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if err := svc.SeedAdd(body.URL); err != nil {
			writeErr(err)
			return
		}
		writeOK(map[string]string{"ok": "1"})
	case path == "health" && r.Method == http.MethodGet:
		h, err := svc.Health()
		if err != nil {
			writeErr(err)
			return
		}
		writeOK(h)
	case path == "start" && r.Method == http.MethodPost:
		if err := svc.StartMesh(); err != nil {
			writeErr(err)
			return
		}
		writeOK(map[string]string{"ok": "1"})
	case path == "stop" && r.Method == http.MethodPost:
		svc.StopMesh()
		writeOK(map[string]string{"ok": "1"})
	case path == "discover" && r.Method == http.MethodPost:
		if err := svc.Discover(); err != nil {
			writeErr(err)
			return
		}
		writeOK(map[string]string{"ok": "1"})
	case path == "peers" && r.Method == http.MethodGet:
		p, d, err := svc.Peers()
		if err != nil {
			writeErr(err)
			return
		}
		writeOK(map[string]any{"punched": p, "discovery": d})
	case path == "send" && r.Method == http.MethodPost:
		var body struct {
			To   string `json:"to"`
			Text string `json:"text"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		pkt, err := svc.Send(body.To, body.Text)
		if err != nil {
			writeErr(err)
			return
		}
		writeOK(map[string]string{"sha1": pkt.PacketSHA1})
	case path == "broadcast" && r.Method == http.MethodPost:
		var body struct {
			Text  string `json:"text"`
			Alert bool   `json:"alert"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		pkt, err := svc.Broadcast(body.Text, body.Alert)
		if err != nil {
			writeErr(err)
			return
		}
		writeOK(map[string]string{"sha1": pkt.PacketSHA1})
	case path == "inbox" && r.Method == http.MethodGet:
		msgs, err := svc.Inbox()
		if err != nil {
			writeOK([]any{})
			return
		}
		type row struct {
			From string `json:"from"`
			Text string `json:"text"`
			Tag  string `json:"tag"`
			Via  string `json:"via"`
		}
		var out []row
		for _, m := range msgs {
			tag := "dm"
			if m.Broadcast {
				tag = "bc"
			}
			if m.Alert {
				tag = "alert"
			}
			out = append(out, row{From: m.FromID, Text: m.Text, Tag: tag, Via: m.Via})
		}
		writeOK(out)
	case path == "inbox/clear" && r.Method == http.MethodPost:
		svc.ClearInbox()
		writeOK(map[string]string{"ok": "1"})
	case path == "block" && r.Method == http.MethodPost:
		var body struct {
			ID string `json:"id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if err := svc.Block(body.ID); err != nil {
			writeErr(err)
			return
		}
		writeOK(map[string]string{"ok": "1"})
	case path == "verify" && r.Method == http.MethodPost:
		var body struct {
			ID string `json:"id"`
			FP string `json:"fp"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if err := svc.VerifyFingerprint(body.ID, body.FP); err != nil {
			writeErr(err)
			return
		}
		writeOK(map[string]string{"ok": "1"})
	case path == "trust" && r.Method == http.MethodGet:
		writeOK(svc.TrustList())
	case path == "chain" && r.Method == http.MethodGet:
		// Ensure mesh so chain status is available
		if err := svc.EnsureMesh(); err != nil {
			writeErr(err)
			return
		}
		writeOK(svc.ChainStatus())
	case path == "chain/transfer" && r.Method == http.MethodPost:
		var body struct {
			To     string `json:"to"`
			Amount uint64 `json:"amount"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(err)
			return
		}
		body.To = strings.TrimSpace(body.To)
		if body.To == "" || body.Amount == 0 {
			writeErr(fmt.Errorf("to and amount required (amount is integer MST)"))
			return
		}
		if err := svc.ChainTransfer(body.To, body.Amount); err != nil {
			writeErr(err)
			return
		}
		writeOK(map[string]any{"ok": true, "to": body.To, "amount": body.Amount})
	case path == "chain/mine" && r.Method == http.MethodPost:
		// GUI / solo operators: force propose when eligible (bootstrap-friendly)
		b, err := svc.ChainMine(true)
		if err != nil {
			writeErr(err)
			return
		}
		writeOK(map[string]any{
			"ok":     true,
			"height": b.Header.Height,
			"slot":   b.Header.Slot,
			"hash":   b.Header.HashHex(),
			"txs":    len(b.Txs),
		})
	case path == "chain/stake" && r.Method == http.MethodPost:
		var body struct {
			Amount uint64 `json:"amount"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Amount == 0 {
			writeErr(fmt.Errorf("amount required"))
			return
		}
		if err := svc.ChainStake(body.Amount); err != nil {
			writeErr(err)
			return
		}
		writeOK(map[string]any{"ok": true, "amount": body.Amount})
	case path == "chain/activate" && r.Method == http.MethodPost:
		if err := svc.ChainActivate(); err != nil {
			writeErr(err)
			return
		}
		writeOK(map[string]string{"ok": "1"})
	default:
		w.WriteHeader(404)
		writeErr(fmt.Errorf("not found: %s", path))
	}
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}
