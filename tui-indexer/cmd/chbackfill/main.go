// Command chbackfill reads pubnet ledgers from the Stellar S3 data lake and
// inserts core rows into ClickHouse. Research harness only.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/stellar/go-stellar-sdk/ingest"
	"github.com/stellar/go-stellar-sdk/ingest/ledgerbackend"
	"github.com/stellar/go-stellar-sdk/network"

	"github.com/miguelnietoa/stellar-explorer/tui-indexer/internal/source"
	"github.com/miguelnietoa/stellar-explorer/tui-indexer/internal/store"
	"github.com/miguelnietoa/stellar-explorer/tui-indexer/internal/transform"
)

func main() {
	start := flag.Uint("start", 0, "start ledger (inclusive)")
	end := flag.Uint("end", 0, "end ledger (inclusive)")
	workers := flag.Int("workers", 8, "parallel S3 workers")
	dsn := flag.String("dsn", envOr("CLICKHOUSE_DSN", "clickhouse://default:@localhost:9000/stellar_benchmark"), "ClickHouse DSN")
	flag.Parse()

	if *start == 0 || *end == 0 || *end < *start {
		fmt.Fprintf(os.Stderr, "Usage: chbackfill --start <ledger> --end <ledger> [--workers N] [--dsn DSN]\n")
		os.Exit(2)
	}

	ctx := context.Background()
	conn, err := openClickHouse(ctx, *dsn)
	if err != nil {
		log.Fatalf("clickhouse: %v", err)
	}
	if err := conn.Ping(ctx); err != nil {
		log.Fatalf("clickhouse ping: %v", err)
	}
	_ = conn.Close()

	startLedger := uint32(*start)
	endLedger := uint32(*end)
	if startLedger < 3 {
		startLedger = 3
	}

	total := endLedger - startLedger + 1
	log.Printf("chbackfill: ledgers %d-%d (%d) workers=%d", startLedger, endLedger, total, *workers)

	w := *workers
	if int(total) < w {
		w = int(total)
	}
	chunk := total / uint32(w)

	var processed atomic.Int64
	var wg sync.WaitGroup
	errCh := make(chan error, w)
	started := time.Now()

	for i := 0; i < w; i++ {
		wg.Add(1)
		ws := startLedger + uint32(i)*chunk
		we := ws + chunk - 1
		if i == w-1 {
			we = endLedger
		}
		go func(id int, from, to uint32) {
			defer wg.Done()
			workerConn, err := openClickHouse(ctx, *dsn)
			if err != nil {
				errCh <- fmt.Errorf("worker %d clickhouse: %w", id, err)
				return
			}
			defer workerConn.Close()
			if err := runWorker(ctx, workerConn, id, from, to, &processed); err != nil && ctx.Err() == nil {
				errCh <- fmt.Errorf("worker %d: %w", id, err)
			}
		}(i, ws, we)
	}

	wg.Wait()
	close(errCh)
	elapsed := time.Since(started)

	var first error
	for err := range errCh {
		if first == nil {
			first = err
		}
		log.Printf("error: %v", err)
	}

	n := processed.Load()
	rate := float64(n) / elapsed.Seconds()
	log.Printf("chbackfill: done processed=%d elapsed=%s ledgers_per_sec=%.2f", n, elapsed.Round(time.Millisecond), rate)
	if first != nil {
		os.Exit(1)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func openClickHouse(ctx context.Context, dsn string) (driver.Conn, error) {
	opts, err := clickhouse.ParseDSN(dsn)
	if err != nil {
		return nil, err
	}
	conn, err := clickhouse.Open(opts)
	if err != nil {
		return nil, err
	}
	if err := conn.Ping(ctx); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return conn, nil
}

func runWorker(ctx context.Context, conn driver.Conn, id int, start, end uint32, processed *atomic.Int64) error {
	log.Printf("chbackfill worker %d: %d-%d", id, start, end)

	ds, err := source.NewAnonymousPubnetDataStore(ctx)
	if err != nil {
		return fmt.Errorf("datastore: %w", err)
	}
	schema := source.PubnetDataLakeConfig().Schema
	bufConfig := ingest.DefaultBufferedStorageBackendConfig(schema.LedgersPerFile)
	backend, err := ledgerbackend.NewBufferedStorageBackend(bufConfig, ds, schema)
	if err != nil {
		return fmt.Errorf("backend: %w", err)
	}
	defer backend.Close()

	if err := backend.PrepareRange(ctx, ledgerbackend.BoundedRange(start, end)); err != nil {
		return fmt.Errorf("prepare: %w", err)
	}

	passphrase := network.PublicNetworkPassphrase
	var ledgers []store.Ledger
	var txs []store.Transaction
	var ops []store.Operation
	var events []store.ContractEvent

	flush := func() error {
		if err := insertLedgers(ctx, conn, ledgers); err != nil {
			return err
		}
		if err := insertTransactions(ctx, conn, txs); err != nil {
			return err
		}
		if err := insertOperations(ctx, conn, ops); err != nil {
			return err
		}
		if err := insertContractEvents(ctx, conn, events); err != nil {
			return err
		}
		ledgers = ledgers[:0]
		txs = txs[:0]
		ops = ops[:0]
		events = events[:0]
		return nil
	}

	for seq := start; seq <= end; seq++ {
		lcm, err := backend.GetLedger(ctx, seq)
		if err != nil {
			return fmt.Errorf("get ledger %d: %w", seq, err)
		}
		ledgerEntry, err := source.LedgerEntryFromCloseMeta(lcm)
		if err != nil {
			return fmt.Errorf("convert ledger %d: %w", seq, err)
		}
		txEntries, err := source.TransactionEntriesFromCloseMeta(lcm, passphrase)
		if err != nil {
			return fmt.Errorf("convert txs %d: %w", seq, err)
		}

		ledger, err := transform.LedgerFromRPC(ledgerEntry)
		if err != nil {
			return fmt.Errorf("transform ledger %d: %w", seq, err)
		}
		ledger.TransactionCount = int32(len(txEntries))
		var success, fail int32
		for _, tx := range txEntries {
			if tx.Status == "SUCCESS" {
				success++
			} else {
				fail++
			}
		}
		ledger.SuccessfulTxCount = success
		ledger.FailedTxCount = fail

		var storeTxs []store.Transaction
		var storeOps []store.Operation
		var opCount int32
		for _, txEntry := range txEntries {
			tx, err := transform.TransactionFromRPC(txEntry, passphrase)
			if err != nil {
				continue
			}
			storeTxs = append(storeTxs, *tx)
			batchOps, err := transform.OperationsFromRPC(txEntry, passphrase)
			if err != nil {
				continue
			}
			storeOps = append(storeOps, batchOps...)
			opCount += int32(len(batchOps))

			ces, err := transform.ContractEventsFromTransaction(txEntry, passphrase)
			if err == nil {
				events = append(events, ces...)
			}
		}
		ledger.OperationCount = opCount
		ledgers = append(ledgers, *ledger)
		txs = append(txs, storeTxs...)
		ops = append(ops, storeOps...)

		if len(ledgers) >= 100 || len(txs) >= 2000 || len(ops) >= 4000 {
			if err := flush(); err != nil {
				return err
			}
		}

		n := processed.Add(1)
		if n%1000 == 0 {
			log.Printf("chbackfill worker %d: at %d (total %d)", id, seq, n)
		}
	}
	if err := flush(); err != nil {
		return err
	}
	log.Printf("chbackfill worker %d: done", id)
	return nil
}

func insertLedgers(ctx context.Context, conn driver.Conn, rows []store.Ledger) error {
	if len(rows) == 0 {
		return nil
	}
	batch, err := conn.PrepareBatch(ctx, `INSERT INTO ledgers (
		sequence, hash, prev_hash, closed_at, total_coins, fee_pool, base_fee, base_reserve,
		max_tx_set_size, protocol_version, transaction_count, operation_count,
		successful_tx_count, failed_tx_count, tx_set_operation_count, header_xdr
	)`)
	if err != nil {
		return err
	}
	for _, r := range rows {
		var header string
		if r.HeaderXDR != nil {
			header = *r.HeaderXDR
		}
		if err := batch.Append(
			r.Sequence, r.Hash, r.PrevHash, r.ClosedAt, r.TotalCoins, r.FeePool,
			r.BaseFee, r.BaseReserve, r.MaxTxSetSize, r.ProtocolVersion,
			r.TransactionCount, r.OperationCount, r.SuccessfulTxCount, r.FailedTxCount,
			r.TxSetOperationCount, header,
		); err != nil {
			return err
		}
	}
	return batch.Send()
}

func insertTransactions(ctx context.Context, conn driver.Conn, rows []store.Transaction) error {
	if len(rows) == 0 {
		return nil
	}
	batch, err := conn.PrepareBatch(ctx, `INSERT INTO transactions (
		hash, ledger_sequence, application_order, account, account_muxed, account_muxed_id,
		account_sequence, fee_charged, max_fee, operation_count, memo_type, memo_text, memo_hash,
		status, is_soroban, soroban_resources, envelope_xdr, result_xdr, result_meta_xdr, fee_meta_xdr, created_at
	)`)
	if err != nil {
		return err
	}
	for _, r := range rows {
		var isSoroban uint8
		if r.IsSoroban {
			isSoroban = 1
		}
		if err := batch.Append(
			r.Hash, r.LedgerSequence, r.ApplicationOrder, r.Account, r.AccountMuxed, r.AccountMuxedID,
			r.AccountSequence, r.FeeCharged, r.MaxFee, r.OperationCount, r.MemoType, r.MemoText, r.MemoHash,
			r.Status, isSoroban, r.SorobanResources, r.EnvelopeXDR, r.ResultXDR, r.ResultMetaXDR, r.FeeMetaXDR, r.CreatedAt,
		); err != nil {
			return err
		}
	}
	return batch.Send()
}

func insertOperations(ctx context.Context, conn driver.Conn, rows []store.Operation) error {
	if len(rows) == 0 {
		return nil
	}
	batch, err := conn.PrepareBatch(ctx, `INSERT INTO operations (
		transaction_hash, application_order, type, type_name, source_account,
		asset_code, asset_issuer, amount, destination, contract_id, function_name, details, created_at
	)`)
	if err != nil {
		return err
	}
	for _, r := range rows {
		if err := batch.Append(
			r.TransactionHash, r.ApplicationOrder, r.Type, r.TypeName, r.SourceAccount,
			r.AssetCode, r.AssetIssuer, r.Amount, r.Destination, r.ContractID, r.FunctionName, r.Details, r.CreatedAt,
		); err != nil {
			return err
		}
	}
	return batch.Send()
}

func insertContractEvents(ctx context.Context, conn driver.Conn, rows []store.ContractEvent) error {
	if len(rows) == 0 {
		return nil
	}
	batch, err := conn.PrepareBatch(ctx, `INSERT INTO contract_events (
		contract_id, transaction_hash, ledger_sequence, type,
		topic_1, topic_2, topic_3, topic_4, topics_xdr, value_xdr,
		topics_decoded, value_decoded, created_at
	)`)
	if err != nil {
		return err
	}
	for _, r := range rows {
		if err := batch.Append(
			r.ContractID, r.TransactionHash, r.LedgerSequence, r.Type,
			r.Topic1, r.Topic2, r.Topic3, r.Topic4, r.TopicsXDR, r.ValueXDR,
			r.TopicsDecoded, r.ValueDecoded, r.CreatedAt,
		); err != nil {
			return err
		}
	}
	return batch.Send()
}
