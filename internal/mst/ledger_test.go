package mst

import (
	"path/filepath"
	"testing"
)

func TestGenesisConstantsProduction(t *testing.T) {
	if GenesisGrant != 0 {
		t.Fatalf("GenesisGrant=%d want 0 (no free claim)", GenesisGrant)
	}
	if GenesisSupply != 4_200_000 {
		t.Fatalf("GenesisSupply=%d", GenesisSupply)
	}
}

func TestClaimGenesisDisabled(t *testing.T) {
	dir := t.TempDir()
	l, err := Open(filepath.Join(dir, "node"), "testid")
	if err != nil {
		t.Fatal(err)
	}
	l.path = filepath.Join(dir, "mst-ledger.json")
	if err := l.ClaimGenesis(true); err == nil {
		t.Fatal("claim must be disabled")
	}
}
