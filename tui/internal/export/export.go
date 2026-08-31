// Package export writes tabular TUI data (currently the live feed transaction
// list) to CSV or JSON files so users can pull data out of the terminal.
package export

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/miguelnietoa/stellar-explorer/tui/internal/backendclient"
)

// Format identifies a supported export file format.
type Format string

const (
	FormatCSV  Format = "csv"
	FormatJSON Format = "json"
)

var csvHeader = []string{
	"hash",
	"ledger_sequence",
	"application_order",
	"account",
	"operation_count",
	"status",
	"is_soroban",
	"created_at",
	"primary_contract_id",
	"primary_asset_code",
	"primary_asset_issuer",
	"primary_operation_type",
}

// ParseFormat validates a user-supplied format string.
func ParseFormat(value string) (Format, error) {
	switch Format(value) {
	case FormatCSV:
		return FormatCSV, nil
	case FormatJSON:
		return FormatJSON, nil
	default:
		return "", fmt.Errorf("unsupported export format %q", value)
	}
}

// DefaultFileName builds a timestamped file name for the given view and format.
func DefaultFileName(view string, format Format, now time.Time) string {
	return fmt.Sprintf("stellarview-%s-%s.%s", view, now.UTC().Format("20060102-150405"), format)
}

// WriteLiveFeedTransactions writes the given transactions to path in the requested format.
func WriteLiveFeedTransactions(path string, format Format, transactions []backendclient.TransactionSummary) error {
	switch format {
	case FormatCSV:
		return writeLiveFeedCSV(path, transactions)
	case FormatJSON:
		return writeLiveFeedJSON(path, transactions)
	default:
		return fmt.Errorf("unsupported export format %q", format)
	}
}

func writeLiveFeedCSV(path string, transactions []backendclient.TransactionSummary) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create export file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	if err := writer.Write(csvHeader); err != nil {
		return fmt.Errorf("write export header: %w", err)
	}
	for _, tx := range transactions {
		record := []string{
			tx.Hash,
			fmt.Sprintf("%d", tx.LedgerSequence),
			fmt.Sprintf("%d", tx.ApplicationOrder),
			tx.Account,
			fmt.Sprintf("%d", tx.OperationCount),
			fmt.Sprintf("%d", tx.Status),
			fmt.Sprintf("%t", tx.IsSoroban),
			tx.CreatedAt.UTC().Format(time.RFC3339),
			tx.PrimaryContractID,
			tx.PrimaryAssetCode,
			tx.PrimaryAssetIssuer,
			tx.PrimaryOperationType,
		}
		if err := writer.Write(record); err != nil {
			return fmt.Errorf("write export row: %w", err)
		}
	}
	writer.Flush()
	return writer.Error()
}

func writeLiveFeedJSON(path string, transactions []backendclient.TransactionSummary) error {
	if transactions == nil {
		transactions = []backendclient.TransactionSummary{}
	}
	data, err := json.MarshalIndent(transactions, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal export data: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write export file: %w", err)
	}
	return nil
}
