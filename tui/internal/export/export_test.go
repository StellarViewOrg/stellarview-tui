package export

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/miguelnietoa/stellar-explorer/tui/internal/backendclient"
)

func TestParseFormat(t *testing.T) {
	if _, err := ParseFormat("csv"); err != nil {
		t.Fatalf("ParseFormat(csv) error = %v", err)
	}
	if _, err := ParseFormat("json"); err != nil {
		t.Fatalf("ParseFormat(json) error = %v", err)
	}
	if _, err := ParseFormat("xml"); err == nil {
		t.Fatal("expected error for unsupported format")
	}
}

func TestDefaultFileName(t *testing.T) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	got := DefaultFileName("live-feed", FormatCSV, now)
	want := "stellarview-live-feed-20260102-030405.csv"
	if got != want {
		t.Fatalf("DefaultFileName() = %q, want %q", got, want)
	}
}

func TestWriteLiveFeedTransactionsCSV(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.csv")

	txs := []backendclient.TransactionSummary{
		{
			Hash:                 "tx-1",
			LedgerSequence:       10,
			ApplicationOrder:     1,
			Account:              "GACCOUNT",
			OperationCount:       2,
			Status:               1,
			IsSoroban:            true,
			CreatedAt:            time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			PrimaryContractID:    "CCONTRACT",
			PrimaryAssetCode:     "XLM",
			PrimaryAssetIssuer:   "",
			PrimaryOperationType: "invoke_host_function",
		},
	}

	if err := WriteLiveFeedTransactions(path, FormatCSV, txs); err != nil {
		t.Fatalf("WriteLiveFeedTransactions error = %v", err)
	}

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open export file: %v", err)
	}
	defer file.Close()

	records, err := csv.NewReader(file).ReadAll()
	if err != nil {
		t.Fatalf("read csv: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected header + 1 row, got %d rows", len(records))
	}
	if records[0][0] != "hash" {
		t.Fatalf("unexpected header: %v", records[0])
	}
	if records[1][0] != "tx-1" || records[1][3] != "GACCOUNT" {
		t.Fatalf("unexpected row: %v", records[1])
	}
}

func TestWriteLiveFeedTransactionsJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.json")

	txs := []backendclient.TransactionSummary{
		{Hash: "tx-1", Account: "GACCOUNT"},
	}

	if err := WriteLiveFeedTransactions(path, FormatJSON, txs); err != nil {
		t.Fatalf("WriteLiveFeedTransactions error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read export file: %v", err)
	}

	var decoded []backendclient.TransactionSummary
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal export file: %v", err)
	}
	if len(decoded) != 1 || decoded[0].Hash != "tx-1" {
		t.Fatalf("unexpected decoded content: %+v", decoded)
	}
}

func TestWriteLiveFeedTransactionsUnsupportedFormat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.xml")
	if err := WriteLiveFeedTransactions(path, Format("xml"), nil); err == nil {
		t.Fatal("expected error for unsupported format")
	}
}
