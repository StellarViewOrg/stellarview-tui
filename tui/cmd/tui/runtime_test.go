package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/miguelnietoa/stellar-explorer/tui/internal/app"
	"github.com/miguelnietoa/stellar-explorer/tui/internal/backendclient"
	"github.com/miguelnietoa/stellar-explorer/tui/internal/config"
	"github.com/miguelnietoa/stellar-explorer/tui/internal/ui"
)

func TestNormalizeInteractiveKey(t *testing.T) {
	tests := []struct {
		input []byte
		want  interactiveKey
	}{
		{[]byte("q"), keyQuit},
		{[]byte("h"), keyHome},
		{[]byte("l"), keyLiveFeed},
		{[]byte("u"), keyLookup},
		{[]byte("s"), keySettings},
		{[]byte("b"), keyBack},
		{[]byte("f"), keyForward},
		{[]byte("?"), keyHelp},
		{[]byte("r"), keyRefresh},
		{[]byte("/"), keySearch},
		{[]byte("\t"), keyFocusNext},
		{[]byte("j"), keyMoveDown},
		{[]byte("k"), keyMoveUp},
		{[]byte("\x1b[B"), keyMoveDown},
		{[]byte("\x1b[A"), keyMoveUp},
		{[]byte("\r"), keyEnter},
		{[]byte("\x7f"), keyBackspace},
		{[]byte("\x1b"), keyEscape},
	}

	for _, tt := range tests {
		if got := normalizeInteractiveKey(tt.input); got != tt.want {
			t.Fatalf("normalizeInteractiveKey(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestHandleInputOpensAndExecutesCommandPalette(t *testing.T) {
	cfg := config.Default()
	model := app.NewModel(cfg, "/tmp/config.json", app.CacheSnapshot{})
	runtime := interactiveRuntime{}

	keepRunning, err := runtime.handleInput(context.Background(), cfg, model, []byte("/"))
	if err != nil {
		t.Fatalf("handleInput(search) error = %v", err)
	}
	if !keepRunning {
		t.Fatal("expected runtime to keep running")
	}
	if !model.Snapshot().Command.Visible {
		t.Fatal("expected command palette to open")
	}

	for _, chunk := range [][]byte{[]byte("l"), []byte("i"), []byte("v"), []byte("e")} {
		if _, err := runtime.handleInput(context.Background(), cfg, model, chunk); err != nil {
			t.Fatalf("handleInput(text) error = %v", err)
		}
	}
	if _, err := runtime.handleInput(context.Background(), cfg, model, []byte("\r")); err != nil {
		t.Fatalf("handleInput(enter) error = %v", err)
	}

	snapshot := model.Snapshot()
	if snapshot.Command.Visible {
		t.Fatal("expected command palette to close after execution")
	}
	if snapshot.Current != app.ScreenLiveFeed {
		t.Fatalf("expected live-feed screen after executing command, got %q", snapshot.Current)
	}
}

func TestCommandPaletteExecutesInferredTransactionResult(t *testing.T) {
	cfg := config.Default()
	model := app.NewModel(cfg, "/tmp/config.json", app.CacheSnapshot{})
	runtime := interactiveRuntime{}

	previousFactory := openLookupBackend
	t.Cleanup(func() {
		openLookupBackend = previousFactory
	})
	openLookupBackend = func(profile config.Profile) (lookupBackend, error) {
		return stubLookupBackend{
			transaction: backendclient.TransactionLookupResponse{
				Transaction: &backendclient.TransactionDetail{
					Hash:           "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					Account:        "GTEST",
					LedgerSequence: 123,
				},
			},
		}, nil
	}

	if _, err := runtime.handleInput(context.Background(), cfg, model, []byte("/")); err != nil {
		t.Fatalf("open palette: %v", err)
	}
	for _, chunk := range []byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa") {
		if _, err := runtime.handleInput(context.Background(), cfg, model, []byte{chunk}); err != nil {
			t.Fatalf("type tx hash: %v", err)
		}
	}
	if _, err := runtime.handleInput(context.Background(), cfg, model, []byte("\r")); err != nil {
		t.Fatalf("execute inferred result: %v", err)
	}

	snapshot := model.Snapshot()
	if snapshot.Lookup.Transaction == nil || snapshot.Lookup.Transaction.Transaction == nil {
		t.Fatal("expected inferred transaction lookup to execute")
	}
	if snapshot.Lookup.Transaction.Transaction.Hash != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("unexpected transaction hash %q", snapshot.Lookup.Transaction.Transaction.Hash)
	}
}

func TestClipboardSelectionReturnsLiveFeedHash(t *testing.T) {
	model := app.NewModel(config.Default(), "/tmp/config.json", app.CacheSnapshot{})
	if err := model.SetScreen(app.ScreenLiveFeed); err != nil {
		t.Fatalf("SetScreen() error = %v", err)
	}
	if err := model.RefreshLiveFeed(context.Background(), stubLiveFeedService{
		summary: backendclient.LiveFeedSummaryResponse{
			RecentTransactions: []backendclient.TransactionSummary{
				{Hash: "tx-1"},
				{Hash: "tx-2"},
			},
		},
	}); err != nil {
		t.Fatalf("RefreshLiveFeed() error = %v", err)
	}
	model.MoveSelection(1)

	if got := clipboardSelection(model.Snapshot(), nil); got != "tx-2" {
		t.Fatalf("clipboardSelection() = %q, want %q", got, "tx-2")
	}
}

func TestClipboardSelectionUsesLiveFeedCopyField(t *testing.T) {
	model := app.NewModel(config.Default(), "/tmp/config.json", app.CacheSnapshot{})
	if err := model.SetScreen(app.ScreenLiveFeed); err != nil {
		t.Fatalf("SetScreen() error = %v", err)
	}
	if err := model.RefreshLiveFeed(context.Background(), stubLiveFeedService{
		summary: backendclient.LiveFeedSummaryResponse{
			RecentTransactions: []backendclient.TransactionSummary{
				{Hash: "tx-1", LedgerSequence: 77, Account: "GACCOUNT77"},
			},
		},
	}); err != nil {
		t.Fatalf("RefreshLiveFeed() error = %v", err)
	}

	dashboard := ui.NewDashboardModel(model.Snapshot(), 100, 20)
	updated, _ := dashboard.Update(tea.KeyMsg{Type: tea.KeyRight})
	dashboard = updated.(ui.DashboardModel)
	if got := clipboardSelection(model.Snapshot(), &dashboard); got != "77" {
		t.Fatalf("clipboardSelection() = %q, want %q", got, "77")
	}

	updated, _ = dashboard.Update(tea.KeyMsg{Type: tea.KeyRight})
	dashboard = updated.(ui.DashboardModel)
	if got := clipboardSelection(model.Snapshot(), &dashboard); got != "GACCOUNT77" {
		t.Fatalf("clipboardSelection() = %q, want %q", got, "GACCOUNT77")
	}
}

func TestClipboardSelectionPrefersLookupSectionValue(t *testing.T) {
	model := app.NewModel(config.Default(), "/tmp/config.json", app.CacheSnapshot{})
	model.UpdateLookupTransaction(
		"tx-1",
		backendclient.TransactionLookupResponse{
			Transaction: &backendclient.TransactionDetail{
				Hash:           "tx-1",
				LedgerSequence: 55,
				OperationCount: 2,
				Status:         1,
				Account:        "GSELECTEDACCOUNT",
			},
		},
	)

	dashboard := ui.NewDashboardModel(model.Snapshot(), 100, 20)
	for range 4 {
		updated, _ := dashboard.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
		dashboard = updated.(ui.DashboardModel)
	}

	if got := clipboardSelection(model.Snapshot(), &dashboard); got != "GSELECTEDACCOUNT" {
		t.Fatalf("clipboardSelection() = %q, want %q", got, "GSELECTEDACCOUNT")
	}
}

func TestApplyActionPastesClipboardIntoCommandPalette(t *testing.T) {
	cfg := config.Default()
	model := app.NewModel(cfg, "/tmp/config.json", app.CacheSnapshot{})
	model.OpenCommandPalette("Search / Command", "")
	dashboard := ui.NewDashboardModel(model.Snapshot(), 100, 20)

	previousRead := readClipboard
	readClipboard = func(context.Context) (string, error) {
		return "lookup tx abc123", nil
	}
	t.Cleanup(func() {
		readClipboard = previousRead
	})

	runtime := interactiveRuntime{}
	keepRunning, err := runtime.applyAction(context.Background(), cfg, model, nil, &dashboard, ui.ActionMsg{Kind: ui.ActionPasteClipboard})
	if err != nil {
		t.Fatalf("applyAction(paste) error = %v", err)
	}
	if !keepRunning {
		t.Fatal("expected runtime to keep running")
	}
	if got := dashboard.CommandInput(); got != "lookup tx abc123" {
		t.Fatalf("dashboard.CommandInput() = %q, want %q", got, "lookup tx abc123")
	}
}

func TestApplyActionExportLiveFeedWritesFile(t *testing.T) {
	dir := t.TempDir()
	previousWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(previousWd)
	})

	previousNow := exportNow
	fixedNow := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	exportNow = func() time.Time { return fixedNow }
	t.Cleanup(func() { exportNow = previousNow })

	cfg := config.Default()
	model := app.NewModel(cfg, "/tmp/config.json", app.CacheSnapshot{})
	if err := model.SetScreen(app.ScreenLiveFeed); err != nil {
		t.Fatalf("SetScreen() error = %v", err)
	}
	if err := model.RefreshLiveFeed(context.Background(), stubLiveFeedService{
		summary: backendclient.LiveFeedSummaryResponse{
			RecentTransactions: []backendclient.TransactionSummary{
				{Hash: "tx-1", Account: "GACCOUNT"},
			},
		},
	}); err != nil {
		t.Fatalf("RefreshLiveFeed() error = %v", err)
	}

	dashboard := ui.NewDashboardModel(model.Snapshot(), 100, 20)
	runtime := interactiveRuntime{}
	keepRunning, err := runtime.applyAction(context.Background(), cfg, model, nil, &dashboard, ui.ActionMsg{Kind: ui.ActionExportLiveFeed, Text: "csv"})
	if err != nil {
		t.Fatalf("applyAction(export) error = %v", err)
	}
	if !keepRunning {
		t.Fatal("expected runtime to keep running")
	}

	wantPath := filepath.Join(dir, "stellarview-live-feed-20260102-030405.csv")
	if _, err := os.Stat(wantPath); err != nil {
		t.Fatalf("expected export file at %s: %v", wantPath, err)
	}

	if got := model.Snapshot().Status.Message; got == "" {
		t.Fatal("expected a status message after export")
	}
}

func TestApplyActionExportLiveFeedWithoutTransactionsWarns(t *testing.T) {
	cfg := config.Default()
	model := app.NewModel(cfg, "/tmp/config.json", app.CacheSnapshot{})
	dashboard := ui.NewDashboardModel(model.Snapshot(), 100, 20)
	runtime := interactiveRuntime{}

	if _, err := runtime.applyAction(context.Background(), cfg, model, nil, &dashboard, ui.ActionMsg{Kind: ui.ActionExportLiveFeed, Text: "csv"}); err != nil {
		t.Fatalf("applyAction(export) error = %v", err)
	}

	if got := model.Snapshot().Status.Level; got != app.StatusWarn {
		t.Fatalf("Status.Level = %v, want %v", got, app.StatusWarn)
	}
}

type stubLiveFeedService struct {
	summary backendclient.LiveFeedSummaryResponse
}

func (s stubLiveFeedService) LiveFeedSummary(context.Context) (backendclient.LiveFeedSummaryResponse, error) {
	return s.summary, nil
}
