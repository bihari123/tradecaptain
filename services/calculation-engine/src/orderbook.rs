use std::collections::BTreeMap;
use std::cmp::Ordering;
use ordered_float::OrderedFloat;
use serde::{Deserialize, Serialize};
use std::time::{SystemTime, UNIX_EPOCH};

/// High-performance order book implementation optimized for financial trading
/// Uses BTreeMap for O(log n) operations and cache-optimized data structures
#[derive(Debug, Clone)]
pub struct OrderBook {
    symbol: String,

    // Buy orders (bids) - ordered from highest to lowest price
    bids: BTreeMap<OrderedFloat<f64>, PriceLevel>,

    // Sell orders (asks) - ordered from lowest to highest price
    asks: BTreeMap<OrderedFloat<f64>, PriceLevel>,

    // Fast access to best prices (cached for O(1) lookup)
    best_bid: Option<f64>,
    best_ask: Option<f64>,

    // Sequence number for ordering updates
    sequence: u64,

    // Statistics
    total_bid_volume: f64,
    total_ask_volume: f64,
    last_update_time: u64,
}

/// Price level containing aggregated orders at a specific price
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PriceLevel {
    pub price: f64,
    pub size: f64,
    pub order_count: u32,
    pub timestamp: u64,
}

/// Individual order
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Order {
    pub order_id: String,
    pub side: Side,
    pub price: f64,
    pub quantity: f64,
    pub timestamp: u64,
}

/// Order side
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum Side {
    Buy,
    Sell,
}

/// Level 2 market data snapshot
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Level2Snapshot {
    pub symbol: String,
    pub bids: Vec<PriceLevel>,
    pub asks: Vec<PriceLevel>,
    pub timestamp: u64,
    pub sequence: u64,
}

/// Best bid/offer (Level 1 data)
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct BestBidOffer {
    pub symbol: String,
    pub bid_price: Option<f64>,
    pub bid_size: Option<f64>,
    pub ask_price: Option<f64>,
    pub ask_size: Option<f64>,
    pub spread: Option<f64>,
    pub mid_price: Option<f64>,
    pub timestamp: u64,
}

impl OrderBook {
    /// Create a new order book for a symbol
    pub fn new(symbol: String) -> Self {
        Self {
            symbol,
            bids: BTreeMap::new(),
            asks: BTreeMap::new(),
            best_bid: None,
            best_ask: None,
            sequence: 0,
            total_bid_volume: 0.0,
            total_ask_volume: 0.0,
            last_update_time: current_timestamp_nanos(),
        }
    }

    /// Add an order to the book - O(log n) complexity
    pub fn add_order(&mut self, order: Order) -> Result<(), String> {
        self.sequence += 1;
        self.last_update_time = current_timestamp_nanos();

        let price_key = OrderedFloat(order.price);

        match order.side {
            Side::Buy => {
                let price_level = self.bids.entry(price_key).or_insert_with(|| PriceLevel {
                    price: order.price,
                    size: 0.0,
                    order_count: 0,
                    timestamp: order.timestamp,
                });

                price_level.size += order.quantity;
                price_level.order_count += 1;
                price_level.timestamp = order.timestamp;

                self.total_bid_volume += order.quantity;

                // Update best bid if this is better
                if self.best_bid.is_none() || order.price > self.best_bid.unwrap() {
                    self.best_bid = Some(order.price);
                }
            }
            Side::Sell => {
                let price_level = self.asks.entry(price_key).or_insert_with(|| PriceLevel {
                    price: order.price,
                    size: 0.0,
                    order_count: 0,
                    timestamp: order.timestamp,
                });

                price_level.size += order.quantity;
                price_level.order_count += 1;
                price_level.timestamp = order.timestamp;

                self.total_ask_volume += order.quantity;

                // Update best ask if this is better
                if self.best_ask.is_none() || order.price < self.best_ask.unwrap() {
                    self.best_ask = Some(order.price);
                }
            }
        }

        Ok(())
    }

    /// Remove quantity from a price level - O(log n) complexity
    pub fn remove_quantity(&mut self, side: Side, price: f64, quantity: f64) -> Result<(), String> {
        self.sequence += 1;
        self.last_update_time = current_timestamp_nanos();

        let price_key = OrderedFloat(price);

        match side {
            Side::Buy => {
                if let Some(price_level) = self.bids.get_mut(&price_key) {
                    if price_level.size < quantity {
                        return Err(format!("Insufficient size at price {}: {} < {}", price, price_level.size, quantity));
                    }

                    price_level.size -= quantity;
                    self.total_bid_volume -= quantity;

                    if price_level.size <= 0.0 {
                        self.bids.remove(&price_key);

                        // Update best bid if we removed the best price
                        if Some(price) == self.best_bid {
                            self.best_bid = self.bids.keys().next_back().map(|p| p.into_inner());
                        }
                    }
                } else {
                    return Err(format!("No bid found at price {}", price));
                }
            }
            Side::Sell => {
                if let Some(price_level) = self.asks.get_mut(&price_key) {
                    if price_level.size < quantity {
                        return Err(format!("Insufficient size at price {}: {} < {}", price, price_level.size, quantity));
                    }

                    price_level.size -= quantity;
                    self.total_ask_volume -= quantity;

                    if price_level.size <= 0.0 {
                        self.asks.remove(&price_key);

                        // Update best ask if we removed the best price
                        if Some(price) == self.best_ask {
                            self.best_ask = self.asks.keys().next().map(|p| p.into_inner());
                        }
                    }
                } else {
                    return Err(format!("No ask found at price {}", price));
                }
            }
        }

        Ok(())
    }

    /// Get best bid and offer (Level 1 data) - O(1) complexity
    pub fn get_best_bid_offer(&self) -> BestBidOffer {
        let bid_level = self.best_bid.and_then(|price| {
            self.bids.get(&OrderedFloat(price))
        });

        let ask_level = self.best_ask.and_then(|price| {
            self.asks.get(&OrderedFloat(price))
        });

        let spread = match (self.best_bid, self.best_ask) {
            (Some(bid), Some(ask)) => Some(ask - bid),
            _ => None,
        };

        let mid_price = match (self.best_bid, self.best_ask) {
            (Some(bid), Some(ask)) => Some((bid + ask) / 2.0),
            _ => None,
        };

        BestBidOffer {
            symbol: self.symbol.clone(),
            bid_price: self.best_bid,
            bid_size: bid_level.map(|l| l.size),
            ask_price: self.best_ask,
            ask_size: ask_level.map(|l| l.size),
            spread,
            mid_price,
            timestamp: self.last_update_time,
        }
    }

    /// Get Level 2 market data (order book depth) - O(n) complexity where n is depth
    pub fn get_level2_snapshot(&self, depth: usize) -> Level2Snapshot {
        // Get top bids (highest prices first)
        let bids: Vec<PriceLevel> = self.bids
            .iter()
            .rev() // Reverse to get highest prices first
            .take(depth)
            .map(|(_, level)| level.clone())
            .collect();

        // Get top asks (lowest prices first)
        let asks: Vec<PriceLevel> = self.asks
            .iter()
            .take(depth)
            .map(|(_, level)| level.clone())
            .collect();

        Level2Snapshot {
            symbol: self.symbol.clone(),
            bids,
            asks,
            timestamp: self.last_update_time,
            sequence: self.sequence,
        }
    }

    /// Calculate volume-weighted average price (VWAP) for top N levels
    pub fn calculate_vwap(&self, side: Side, depth: usize) -> Option<f64> {
        let (total_volume, weighted_sum) = match side {
            Side::Buy => {
                self.bids
                    .iter()
                    .rev()
                    .take(depth)
                    .fold((0.0, 0.0), |(vol, sum), (price, level)| {
                        (vol + level.size, sum + price.into_inner() * level.size)
                    })
            }
            Side::Sell => {
                self.asks
                    .iter()
                    .take(depth)
                    .fold((0.0, 0.0), |(vol, sum), (price, level)| {
                        (vol + level.size, sum + price.into_inner() * level.size)
                    })
            }
        };

        if total_volume > 0.0 {
            Some(weighted_sum / total_volume)
        } else {
            None
        }
    }

    /// Get order book statistics
    pub fn get_statistics(&self) -> OrderBookStats {
        OrderBookStats {
            symbol: self.symbol.clone(),
            bid_levels: self.bids.len(),
            ask_levels: self.asks.len(),
            total_bid_volume: self.total_bid_volume,
            total_ask_volume: self.total_ask_volume,
            spread_bps: self.get_spread_bps(),
            mid_price: self.get_best_bid_offer().mid_price,
            sequence: self.sequence,
            last_update_time: self.last_update_time,
        }
    }

    /// Get spread in basis points (bps)
    fn get_spread_bps(&self) -> Option<f64> {
        match (self.best_bid, self.best_ask) {
            (Some(bid), Some(ask)) => {
                let mid = (bid + ask) / 2.0;
                if mid > 0.0 {
                    Some(((ask - bid) / mid) * 10000.0) // Convert to basis points
                } else {
                    None
                }
            }
            _ => None,
        }
    }

    /// Clear all orders from the book
    pub fn clear(&mut self) {
        self.bids.clear();
        self.asks.clear();
        self.best_bid = None;
        self.best_ask = None;
        self.total_bid_volume = 0.0;
        self.total_ask_volume = 0.0;
        self.sequence += 1;
        self.last_update_time = current_timestamp_nanos();
    }

    /// Get the symbol
    pub fn symbol(&self) -> &str {
        &self.symbol
    }

    /// Get current sequence number
    pub fn sequence(&self) -> u64 {
        self.sequence
    }
}

/// Order book statistics
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct OrderBookStats {
    pub symbol: String,
    pub bid_levels: usize,
    pub ask_levels: usize,
    pub total_bid_volume: f64,
    pub total_ask_volume: f64,
    pub spread_bps: Option<f64>,
    pub mid_price: Option<f64>,
    pub sequence: u64,
    pub last_update_time: u64,
}

/// Multi-symbol order book manager
pub struct OrderBookManager {
    books: BTreeMap<String, OrderBook>,
}

impl OrderBookManager {
    pub fn new() -> Self {
        Self {
            books: BTreeMap::new(),
        }
    }

    /// Get or create order book for symbol
    pub fn get_or_create_book(&mut self, symbol: &str) -> &mut OrderBook {
        self.books.entry(symbol.to_string())
            .or_insert_with(|| OrderBook::new(symbol.to_string()))
    }

    /// Get order book for symbol
    pub fn get_book(&self, symbol: &str) -> Option<&OrderBook> {
        self.books.get(symbol)
    }

    /// Get all symbols
    pub fn get_symbols(&self) -> Vec<String> {
        self.books.keys().cloned().collect()
    }

    /// Add order to appropriate book
    pub fn add_order(&mut self, order: Order) -> Result<(), String> {
        let symbol = match &order.side {
            Side::Buy | Side::Sell => {
                // Extract symbol from order context or pass it separately
                // For now, we'll need the symbol to be provided
                return Err("Symbol must be provided with order".to_string());
            }
        };
    }

    /// Process market data update
    pub fn process_market_data(&mut self, symbol: &str, bids: Vec<PriceLevel>, asks: Vec<PriceLevel>) {
        let book = self.get_or_create_book(symbol);

        // Clear existing levels and rebuild
        book.clear();

        // Add all bids
        for bid in bids {
            let order = Order {
                order_id: format!("bid_{}", bid.price),
                side: Side::Buy,
                price: bid.price,
                quantity: bid.size,
                timestamp: bid.timestamp,
            };
            let _ = book.add_order(order);
        }

        // Add all asks
        for ask in asks {
            let order = Order {
                order_id: format!("ask_{}", ask.price),
                side: Side::Sell,
                price: ask.price,
                quantity: ask.size,
                timestamp: ask.timestamp,
            };
            let _ = book.add_order(order);
        }
    }
}

/// Get current timestamp in nanoseconds
fn current_timestamp_nanos() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap()
        .as_nanos() as u64
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::time::Instant;

    #[test]
    fn test_order_book_basic_operations() {
        let mut book = OrderBook::new("AAPL".to_string());

        // Add some orders
        let buy_order = Order {
            order_id: "buy1".to_string(),
            side: Side::Buy,
            price: 150.00,
            quantity: 100.0,
            timestamp: current_timestamp_nanos(),
        };

        let sell_order = Order {
            order_id: "sell1".to_string(),
            side: Side::Sell,
            price: 150.05,
            quantity: 200.0,
            timestamp: current_timestamp_nanos(),
        };

        book.add_order(buy_order).unwrap();
        book.add_order(sell_order).unwrap();

        let bbo = book.get_best_bid_offer();
        assert_eq!(bbo.bid_price, Some(150.00));
        assert_eq!(bbo.ask_price, Some(150.05));
        assert_eq!(bbo.bid_size, Some(100.0));
        assert_eq!(bbo.ask_size, Some(200.0));
        assert_eq!(bbo.spread, Some(0.05));
    }

    #[test]
    fn test_order_book_performance() {
        let mut book = OrderBook::new("AAPL".to_string());
        let num_orders = 10_000;

        let start = Instant::now();

        // Add orders
        for i in 0..num_orders {
            let buy_order = Order {
                order_id: format!("buy_{}", i),
                side: Side::Buy,
                price: 150.00 - (i as f64) * 0.01,
                quantity: 100.0,
                timestamp: current_timestamp_nanos(),
            };

            let sell_order = Order {
                order_id: format!("sell_{}", i),
                side: Side::Sell,
                price: 150.05 + (i as f64) * 0.01,
                quantity: 100.0,
                timestamp: current_timestamp_nanos(),
            };

            book.add_order(buy_order).unwrap();
            book.add_order(sell_order).unwrap();
        }

        let duration = start.elapsed();
        let ops_per_sec = (num_orders * 2) as f64 / duration.as_secs_f64();

        println!("Order book performance: {:.0} orders/sec", ops_per_sec);

        // Should handle at least 100K orders per second
        assert!(ops_per_sec > 100_000.0);

        // Test Level 2 snapshot performance
        let start = Instant::now();
        let _snapshot = book.get_level2_snapshot(10);
        let snapshot_duration = start.elapsed();

        println!("Level 2 snapshot: {:?}", snapshot_duration);
        assert!(snapshot_duration.as_micros() < 100); // Less than 100 microseconds
    }

    #[test]
    fn test_order_book_spread_calculation() {
        let mut book = OrderBook::new("AAPL".to_string());

        let buy_order = Order {
            order_id: "buy1".to_string(),
            side: Side::Buy,
            price: 100.00,
            quantity: 100.0,
            timestamp: current_timestamp_nanos(),
        };

        let sell_order = Order {
            order_id: "sell1".to_string(),
            side: Side::Sell,
            price: 100.10,
            quantity: 100.0,
            timestamp: current_timestamp_nanos(),
        };

        book.add_order(buy_order).unwrap();
        book.add_order(sell_order).unwrap();

        let stats = book.get_statistics();
        assert!(stats.spread_bps.is_some());

        // 0.10 spread on 100.05 mid = 0.999 bps ≈ 10 bps
        let spread_bps = stats.spread_bps.unwrap();
        assert!((spread_bps - 10.0).abs() < 1.0); // Within 1 bps tolerance
    }
}