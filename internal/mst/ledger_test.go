package mst

import (
	"path/filepath"
	"testing"
)

func TestGenesisConstantsDaiban7(t *testing.T) {
	if GenesisGrant != 128 {
		t.Fatalf("GenesisGrant=%d want 128", GenesisGrant)
	}
	if GenesisSupply != 4_226_880 {
		t.Fatalf("GenesisSupply=%d want 4226880", GenesisSupply)
	}
}

func TestClaimGenesisGrant(t *testing.T) {
	dir := t.TempDir()
	l, err := Open(filepath.Join(dir, "node"), "testid")
	if err != nil {
		t.Fatal(err)
	}
	// force path under temp
	l.path = filepath.Join(dir, "mst-ledger.json")
	if err := l.ClaimGenesis(true); err != nil {
		t.Fatal(err)
	}
	bal, burned, claimed := l.Snapshot()
	if !claimed || bal != GenesisGrant || burned != 0 {
		t.Fatalf("bal=%d burned=%d claimed=%v", bal, burned, claimed)
	}
	if err := l.ClaimGenesis(true); err == nil {
		t.Fatal("double claim should fail")
	}
}
