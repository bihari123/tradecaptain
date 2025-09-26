package messaging

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/lirm/aeron-go/aeron"
	"github.com/lirm/aeron-go/aeron/atomic"
	"github.com/lirm/aeron-go/aeron/logbuffer"
	"github.com/dgraph-io/badger/v3"
	"tradecaptain/data-collector/internal/models"
)

// AeronMessaging provides ultra-low latency messaging with persistence
type AeronMessaging struct {
	context          *aeron.Context
	aeron            *aeron.Aeron
	publication      *aeron.Publication
	subscription     *aeron.Subscription
	fragmentAssembly *aeron.FragmentAssembler
	wal              *badger.DB
	done             chan struct{}
	wg               sync.WaitGroup

	// Performance metrics
	messagesSent     *atomic.Int64
	messagesReceived *atomic.Int64
	totalLatency     *atomic.Int64
}

// NewAeronMessaging creates a new Aeron messaging instance
func NewAeronMessaging(mediaDriverDir string, walPath string) (*AeronMessaging, error) {
	// Configure Aeron for maximum performance
	ctx := aeron.NewContext()
	ctx.AeronDir(mediaDriverDir)

	// Optimize for low latency
	ctx.PublicationConnectionTimeout(5 * time.Second)
	ctx.ImageLivenessTimeout(10 * time.Second)
	ctx.PublicationLingerTimeout(5 * time.Second)

	// Create Aeron instance
	a, err := aeron.Connect(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Aeron: %w", err)
	}

	// Configure BadgerDB for WAL
	opts := badger.DefaultOptions(walPath).
		WithSyncWrites(false).        // Async for maximum speed
		WithCompression(badger.ZSTD). // Fast compression
		WithMemTableSize(32 << 20).   // 32MB memory table
		WithValueLogFileSize(128 << 20) // 128MB value log

	walDB, err := badger.Open(opts)
	if err != nil {
		a.Close()
		return nil, fmt.Errorf("failed to open WAL: %w", err)
	}

	am := &AeronMessaging{
		context:          ctx,
		aeron:            a,
		wal:              walDB,
		done:             make(chan struct{}),
		messagesSent:     atomic.NewInt64(0),
		messagesReceived: atomic.NewInt64(0),
		totalLatency:     atomic.NewInt64(0),
	}

	// Create fragment assembler for handling fragmented messages
	am.fragmentAssembly = aeron.NewFragmentAssembler(am.handleMessage, 4096)

	return am, nil
}

// StartPublisher creates a publication for sending messages
func (am *AeronMessaging) StartPublisher(channel string, streamID int32) error {
	pub, err := am.aeron.AddPublication(channel, streamID)
	if err != nil {
		return fmt.Errorf("failed to add publication: %w", err)
	}

	am.publication = pub

	// Wait for publication to be connected
	for !am.publication.IsConnected() {
		time.Sleep(time.Millisecond)
	}

	log.Printf("Aeron publisher started on %s:%d", channel, streamID)
	return nil
}

// StartSubscriber creates a subscription for receiving messages
func (am *AeronMessaging) StartSubscriber(channel string, streamID int32) error {
	sub, err := am.aeron.AddSubscription(channel, streamID)
	if err != nil {
		return fmt.Errorf("failed to add subscription: %w", err)
	}

	am.subscription = sub

	// Start the polling goroutine
	am.wg.Add(1)
	go am.pollMessages()

	log.Printf("Aeron subscriber started on %s:%d", channel, streamID)
	return nil
}

// PublishMarketData sends market data with microsecond latency
func (am *AeronMessaging) PublishMarketData(data *models.MarketData) error {
	startTime := time.Now()

	// Serialize data (could use Cap'n Proto here for even better performance)
	message, err := data.MarshalBinary()
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	// Offer to Aeron (microsecond latency)
	result := am.publication.Offer(message, 0, int32(len(message)), nil)

	switch result {
	case aeron.BackPressured, aeron.AdminAction:
		// Retry with exponential backoff
		for i := 0; i < 10; i++ {
			time.Sleep(time.Microsecond * time.Duration(1<<i))
			result = am.publication.Offer(message, 0, int32(len(message)), nil)
			if result > 0 {
				break
			}
		}
		if result <= 0 {
			return fmt.Errorf("failed to offer message after retries: %d", result)
		}
	case aeron.Closed:
		return fmt.Errorf("publication is closed")
	case aeron.MaxPositionExceeded:
		return fmt.Errorf("max position exceeded")
	}

	// Async persistence to WAL (non-blocking)
	go am.persistToWAL(data, startTime)

	// Update metrics
	am.messagesSent.Inc()
	latency := time.Since(startTime).Nanoseconds()
	am.totalLatency.Add(latency)

	return nil
}

// persistToWAL asynchronously persists data to write-ahead log
func (am *AeronMessaging) persistToWAL(data *models.MarketData, timestamp time.Time) {
	key := make([]byte, 16)
	// Use timestamp + symbol for ordering
	copy(key[:8], []byte(fmt.Sprintf("%016d", timestamp.UnixNano())))
	copy(key[8:], []byte(data.Symbol)[:8])

	value, err := data.MarshalBinary()
	if err != nil {
		log.Printf("Failed to marshal data for WAL: %v", err)
		return
	}

	err = am.wal.Update(func(txn *badger.Txn) error {
		return txn.Set(key, value)
	})
	if err != nil {
		log.Printf("Failed to write to WAL: %v", err)
	}
}

// pollMessages continuously polls for incoming messages
func (am *AeronMessaging) pollMessages() {
	defer am.wg.Done()

	for {
		select {
		case <-am.done:
			return
		default:
			// Poll with microsecond precision
			fragmentsRead := am.subscription.Poll(am.fragmentAssembly.OnFragment, 10)
			if fragmentsRead == 0 {
				// Short pause to prevent busy waiting
				time.Sleep(100 * time.Nanosecond)
			}
		}
	}
}

// handleMessage processes incoming Aeron messages
func (am *AeronMessaging) handleMessage(buffer *atomic.Buffer, offset int32, length int32, header *logbuffer.Header) {
	startTime := time.Now()

	// Extract message data
	data := make([]byte, length)
	buffer.GetBytes(offset, data)

	// Deserialize market data
	var marketData models.MarketData
	if err := marketData.UnmarshalBinary(data); err != nil {
		log.Printf("Failed to unmarshal message: %v", err)
		return
	}

	// Process the message (this would be application-specific logic)
	am.processMarketData(&marketData)

	// Update metrics
	am.messagesReceived.Inc()
	latency := time.Since(startTime).Nanoseconds()
	am.totalLatency.Add(latency)
}

// processMarketData handles the received market data
func (am *AeronMessaging) processMarketData(data *models.MarketData) {
	// This is where you'd implement your business logic
	// For example: update caches, trigger calculations, etc.

	// Example: Log high-volume trades
	if data.Volume > 1000000 {
		log.Printf("High volume trade: %s @ %.2f (Volume: %d)",
			data.Symbol, data.Price, data.Volume)
	}
}

// GetPerformanceMetrics returns messaging performance statistics
func (am *AeronMessaging) GetPerformanceMetrics() map[string]interface{} {
	sent := am.messagesSent.Get()
	received := am.messagesReceived.Get()
	totalLatency := am.totalLatency.Get()

	var avgLatency float64
	if sent > 0 {
		avgLatency = float64(totalLatency) / float64(sent) / 1000.0 // Convert to microseconds
	}

	return map[string]interface{}{
		"messages_sent":           sent,
		"messages_received":       received,
		"avg_latency_microseconds": avgLatency,
		"publication_connected":   am.publication != nil && am.publication.IsConnected(),
		"subscription_connected":  am.subscription != nil && am.subscription.IsConnected(),
	}
}

// GetWALStats returns WAL performance statistics
func (am *AeronMessaging) GetWALStats() (map[string]interface{}, error) {
	lsm, vlog := am.wal.Size()

	stats := map[string]interface{}{
		"lsm_size_bytes":   lsm,
		"vlog_size_bytes":  vlog,
		"total_size_bytes": lsm + vlog,
		"total_size_mb":    (lsm + vlog) / (1024 * 1024),
	}

	return stats, nil
}

// Stop gracefully shuts down the Aeron messaging system
func (am *AeronMessaging) Stop() error {
	close(am.done)

	// Stop polling
	am.wg.Wait()

	// Close Aeron resources
	if am.publication != nil {
		am.publication.Close()
	}
	if am.subscription != nil {
		am.subscription.Close()
	}
	if am.aeron != nil {
		am.aeron.Close()
	}

	// Close WAL
	if am.wal != nil {
		if err := am.wal.Close(); err != nil {
			return fmt.Errorf("failed to close WAL: %w", err)
		}
	}

	log.Println("Aeron messaging system stopped")
	return nil
}

// RecoverFromWAL replays messages from the write-ahead log
func (am *AeronMessaging) RecoverFromWAL(since time.Time) error {
	log.Println("Starting WAL recovery...")

	sinceNano := since.UnixNano()
	recovered := 0

	err := am.wal.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			key := item.Key()

			// Extract timestamp from key
			timestampBytes := key[:8]
			timestamp := int64(0)
			for i, b := range timestampBytes {
				timestamp |= int64(b) << (8 * (7 - i))
			}

			if timestamp >= sinceNano {
				err := item.Value(func(val []byte) error {
					var data models.MarketData
					if err := data.UnmarshalBinary(val); err != nil {
						return err
					}

					// Republish the message
					return am.PublishMarketData(&data)
				})
				if err != nil {
					return err
				}
				recovered++
			}
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("WAL recovery failed: %w", err)
	}

	log.Printf("WAL recovery completed: %d messages recovered", recovered)
	return nil
}