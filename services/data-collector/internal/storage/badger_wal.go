package storage

import (
	"context"
	"encoding/binary"
	"time"

	"github.com/dgraph-io/badger/v3"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"tradecaptain/data-collector/internal/models"
)

// BadgerWAL provides ultra-fast local write-ahead log with async Kafka replication
type BadgerWAL struct {
	db    *badger.DB
	kafka *kafka.Producer
}

// NewBadgerWAL creates a new BadgerDB-based WAL
func NewBadgerWAL(dbPath string, kafkaBootstrap string) (*BadgerWAL, error) {
	// Configure BadgerDB for maximum performance
	opts := badger.DefaultOptions(dbPath).
		WithSyncWrites(false).        // Async writes for speed
		WithCompression(badger.ZSTD). // Built-in compression
		WithMemTableSize(64 << 20).   // 64MB memory table
		WithValueLogFileSize(256 << 20) // 256MB value log files

	db, err := badger.Open(opts)
	if err != nil {
		return nil, err
	}

	// Configure Kafka producer for async replication
	producer, err := kafka.NewProducer(&kafka.ConfigMap{
		"bootstrap.servers": kafkaBootstrap,
		"acks":             "1",      // Wait for leader acknowledgment
		"batch.size":       "65536",  // 64KB batches
		"linger.ms":        "10",     // 10ms batching
		"compression.type": "lz4",    // Fast compression
	})
	if err != nil {
		db.Close()
		return nil, err
	}

	return &BadgerWAL{
		db:    db,
		kafka: producer,
	}, nil
}

// WriteMarketData writes market data with microsecond latency
func (w *BadgerWAL) WriteMarketData(data *models.MarketData) error {
	// Generate timestamp-based key for ordering
	key := make([]byte, 16)
	binary.BigEndian.PutUint64(key[:8], uint64(time.Now().UnixNano()))
	copy(key[8:], data.Symbol[:8]) // Symbol prefix for partitioning

	// Serialize data (could use MessagePack here for even better performance)
	value, err := data.MarshalBinary()
	if err != nil {
		return err
	}

	// Step 1: Ultra-fast local write (microseconds)
	err = w.db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, value)
	})
	if err != nil {
		return err
	}

	// Step 2: Async Kafka replication (non-blocking)
	go func() {
		w.kafka.Produce(&kafka.Message{
			TopicPartition: kafka.TopicPartition{
				Topic:     &[]string{"market-data"}[0],
				Partition: kafka.PartitionAny,
			},
			Key:   key,
			Value: value,
		}, nil)
	}()

	return nil
}

// BatchWrite writes multiple entries efficiently
func (w *BadgerWAL) BatchWrite(data []*models.MarketData) error {
	wb := w.db.NewWriteBatch()
	defer wb.Cancel()

	for _, item := range data {
		key := make([]byte, 16)
		binary.BigEndian.PutUint64(key[:8], uint64(time.Now().UnixNano()))
		copy(key[8:], item.Symbol[:8])

		value, err := item.MarshalBinary()
		if err != nil {
			return err
		}

		if err := wb.Set(key, value); err != nil {
			return err
		}
	}

	return wb.Flush()
}

// ReadRange reads data within a time range
func (w *BadgerWAL) ReadRange(start, end time.Time) ([]*models.MarketData, error) {
	var results []*models.MarketData

	err := w.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		it := txn.NewIterator(opts)
		defer it.Close()

		startKey := make([]byte, 8)
		endKey := make([]byte, 8)
		binary.BigEndian.PutUint64(startKey, uint64(start.UnixNano()))
		binary.BigEndian.PutUint64(endKey, uint64(end.UnixNano()))

		for it.Seek(startKey); it.Valid(); it.Next() {
			item := it.Item()
			key := item.Key()

			// Check if we've passed the end time
			if binary.BigEndian.Uint64(key[:8]) > binary.BigEndian.Uint64(endKey) {
				break
			}

			err := item.Value(func(val []byte) error {
				var data models.MarketData
				if err := data.UnmarshalBinary(val); err != nil {
					return err
				}
				results = append(results, &data)
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})

	return results, err
}

// Close shuts down the WAL
func (w *BadgerWAL) Close() error {
	w.kafka.Close()
	return w.db.Close()
}

// Stats returns performance statistics
func (w *BadgerWAL) Stats() badger.LSMSize {
	lsm, _ := w.db.Size()
	return lsm
}