package wallet

import (
	"path/filepath"
	"testing"
)

func moreTestDAO(t *testing.T) *DAO {
	t.Helper()
	t.Setenv("LAZYSWAP_TEST", "1")
	dao, err := OpenAt(filepath.Join(t.TempDir(), "wallets.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = dao.Close() })
	return dao
}

func TestGetByAddressAndDelete(t *testing.T) {
	dao := moreTestDAO(t)
	if err := dao.Insert(&Wallet{ID: "w1", Address: "0xAAA", PrivateKey: "enc"}); err != nil {
		t.Fatal(err)
	}

	w, err := dao.GetByAddress("0xAAA")
	if err != nil || w.ID != "w1" {
		t.Fatalf("get: %+v err=%v", w, err)
	}
	if _, err := dao.GetByAddress("0xNope"); err == nil {
		t.Fatal("unknown address must error")
	}

	if err := dao.DeleteByID("w1"); err != nil {
		t.Fatal(err)
	}
	if _, err := dao.GetByAddress("0xAAA"); err == nil {
		t.Fatal("deleted wallet must be gone")
	}
}

func TestConfigRoundTrip(t *testing.T) {
	dao := moreTestDAO(t)

	if _, ok, err := dao.GetConfig("missing"); err != nil || ok {
		t.Fatalf("missing key: ok=%v err=%v", ok, err)
	}
	if err := dao.SetConfig("k", "v1"); err != nil {
		t.Fatal(err)
	}
	if err := dao.SetConfig("k", "v2"); err != nil { // upsert
		t.Fatal(err)
	}
	v, ok, err := dao.GetConfig("k")
	if err != nil || !ok || v != "v2" {
		t.Fatalf("roundtrip: v=%q ok=%v err=%v", v, ok, err)
	}
}
