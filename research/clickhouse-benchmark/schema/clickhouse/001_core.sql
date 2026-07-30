CREATE DATABASE IF NOT EXISTS stellar_benchmark;

CREATE TABLE IF NOT EXISTS stellar_benchmark.ledgers
(
    sequence UInt32,
    hash FixedString(64),
    prev_hash FixedString(64),
    closed_at DateTime64(3, 'UTC'),
    total_coins Int64,
    fee_pool Int64,
    base_fee Int32,
    base_reserve Int32,
    max_tx_set_size Int32,
    protocol_version Int32,
    transaction_count Int32,
    operation_count Int32,
    successful_tx_count Int32,
    failed_tx_count Int32,
    tx_set_operation_count Nullable(Int32),
    header_xdr String,
    ingested_at DateTime64(3, 'UTC') DEFAULT now64(3)
)
ENGINE = ReplacingMergeTree(ingested_at)
ORDER BY (sequence);

CREATE TABLE IF NOT EXISTS stellar_benchmark.transactions
(
    hash FixedString(64),
    ledger_sequence UInt32,
    application_order Int32,
    account String,
    account_muxed Nullable(String),
    account_muxed_id Nullable(Int64),
    account_sequence Int64,
    fee_charged Int64,
    max_fee Int64,
    operation_count Int32,
    memo_type Int16,
    memo_text Nullable(String),
    memo_hash Nullable(String),
    status Int16,
    is_soroban UInt8,
    soroban_resources Nullable(String),
    envelope_xdr String,
    result_xdr String,
    result_meta_xdr Nullable(String),
    fee_meta_xdr Nullable(String),
    created_at DateTime64(3, 'UTC'),
    ingested_at DateTime64(3, 'UTC') DEFAULT now64(3),
    INDEX idx_account account TYPE bloom_filter GRANULARITY 1
)
ENGINE = ReplacingMergeTree(ingested_at)
ORDER BY (hash);

CREATE TABLE IF NOT EXISTS stellar_benchmark.operations
(
    transaction_hash FixedString(64),
    application_order Int32,
    type Int16,
    type_name String,
    source_account Nullable(String),
    asset_code Nullable(String),
    asset_issuer Nullable(String),
    amount Nullable(String),
    destination Nullable(String),
    contract_id Nullable(String),
    function_name Nullable(String),
    details String,
    created_at DateTime64(3, 'UTC'),
    ingested_at DateTime64(3, 'UTC') DEFAULT now64(3),
    INDEX idx_source source_account TYPE bloom_filter GRANULARITY 1,
    INDEX idx_asset asset_code TYPE bloom_filter GRANULARITY 1
)
ENGINE = ReplacingMergeTree(ingested_at)
ORDER BY (transaction_hash, application_order);

CREATE TABLE IF NOT EXISTS stellar_benchmark.contract_events
(
    contract_id String,
    transaction_hash FixedString(64),
    ledger_sequence UInt32,
    type Int16,
    topic_1 Nullable(String),
    topic_2 Nullable(String),
    topic_3 Nullable(String),
    topic_4 Nullable(String),
    topics_xdr String,
    value_xdr String,
    topics_decoded Nullable(String),
    value_decoded Nullable(String),
    created_at DateTime64(3, 'UTC'),
    ingested_at DateTime64(3, 'UTC') DEFAULT now64(3)
)
ENGINE = ReplacingMergeTree(ingested_at)
ORDER BY (contract_id, created_at, transaction_hash, type, topic_1, topic_2, topic_3, topic_4, value_xdr);

CREATE TABLE IF NOT EXISTS stellar_benchmark.accounts
(
    id String,
    sequence Int64,
    balance String,
    updated_at DateTime64(3, 'UTC'),
    ingested_at DateTime64(3, 'UTC') DEFAULT now64(3)
)
ENGINE = ReplacingMergeTree(ingested_at)
ORDER BY (id);
