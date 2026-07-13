// One-shot P2P + full stack smoke test.
package main

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/YizeHe/MSP-project/internal/config"
	"github.com/YizeHe/MSP-project/internal/crypto"
	"github.com/YizeHe/MSP-project/internal/identity"
	"github.com/YizeHe/MSP-project/internal/mst"
	"github.com/YizeHe/MSP-project/internal/node"
	"github.com/YizeHe/MSP-project/internal/seedclient"
	"github.com/YizeHe/MSP-project/internal/trust"
)

func main() {
	_ = os.Setenv("MSP_POW_FAST", "1")
	seedURL := env("MSP_SEED", "https://msp.forbiddenx.top")
	proxyURL := env("MSP_PROXY", "")
	fmt.Println("seed:", seedURL, "proxy:", proxyURL)

	idA, err := identity.Generate()
	must(err)
	idB, err := identity.Generate()
	must(err)
	fmt.Println("A", idA.NodeID)
	fmt.Println("B", idB.NodeID)

	dirA, dirB := "msp-data-tA", "msp-data-tB"
	_ = os.RemoveAll(dirA)
	_ = os.RemoveAll(dirB)
	_ = os.MkdirAll(dirA, 0o700)
	_ = os.MkdirAll(dirB, 0o700)

	ta, _ := trust.Open(dirA + "/trust.json")
	tb, _ := trust.Open(dirB + "/trust.json")
	la, _ := mst.Open(dirA, idA.NodeID)
	lb, _ := mst.Open(dirB, idB.NodeID)
	ba, _ := crypto.GenerateBundle(idA.EdPrivate(), idA.XPrivate())
	bb, _ := crypto.GenerateBundle(idB.EdPrivate(), idB.XPrivate())

	seedA := seedclient.New(seedURL)
	seedB := seedclient.New(seedURL)
	if proxyURL != "" {
		u, err := url.Parse(proxyURL)
		must(err)
		seedB.HTTP = &http.Client{Timeout: 30 * time.Second, Transport: &http.Transport{Proxy: http.ProxyURL(u)}}
	}

	opt := func(id *identity.Identity, seed *seedclient.Client, tr *trust.Store, led *mst.Ledger, bun *crypto.PreKeyBundle, dir string) node.Options {
		return node.Options{
			PoWBits: 2, EnforceClock: false, ForwardDelay: false, PadDatagram: true,
			Trust: tr, Ledger: led, Bundle: bun, DataDir: dir, ExtraSeeds: []string{seedURL},
			Log: func(s string) { fmt.Println("["+id.NodeID[:8]+"]", s) },
		}
	}

	nA, err := node.Start(idA, seedA, opt(idA, seedA, ta, la, ba, dirA))
	must(err)
	defer nA.Close()
	nB, err := node.Start(idB, seedB, opt(idB, seedB, tb, lb, bb, dirB))
	must(err)
	defer nB.Close()

	must(nA.ClaimGenesisMST())
	must(nB.ClaimGenesisMST())
	_, err = nA.Join()
	must(err)
	time.Sleep(400 * time.Millisecond)
	_, err = nB.Join()
	must(err)

	for i := 0; i < 8; i++ {
		_ = nA.DiscoverOnce()
		_ = nB.DiscoverOnce()
		if nA.Mesh.ConnectedCount() >= 1 && nB.Mesh.ConnectedCount() >= 1 {
			break
		}
		time.Sleep(400 * time.Millisecond)
	}
	fmt.Println("neighbors", nA.Mesh.ConnectedCount(), nB.Mesh.ConnectedCount())
	if nA.Mesh.ConnectedCount() < 1 || nB.Mesh.ConnectedCount() < 1 {
		fmt.Println("FAIL punch")
		os.Exit(2)
	}
	_ = nA.FloodKeyBundle()
	_ = nB.FloodKeyBundle()
	time.Sleep(500 * time.Millisecond)

	_, err = nA.SendUnicast(idB.NodeID, "hello-x3dh-p2p", time.Hour)
	must(err)
	_, err = nB.SendBroadcast("bc-full", time.Hour, false)
	must(err)
	time.Sleep(1500 * time.Millisecond)

	got := false
	for _, m := range nB.Inbox() {
		if m.Text == "hello-x3dh-p2p" {
			got = true
		}
	}
	gotBC := false
	for _, m := range nA.Inbox() {
		if m.Text == "bc-full" {
			gotBC = true
		}
	}
	fmt.Println("unicast", got, "broadcast", gotBC)
	fmt.Println("builtin seeds", config.BuiltInSeeds)
	if !got || !gotBC {
		os.Exit(1)
	}
	fmt.Println("PASS full stack")
}

func must(err error) {
	if err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}
}
func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
