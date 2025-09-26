# Cap'n Proto schema for ultra-low latency HFT messaging
# Optimized for zero-copy serialization and minimal memory bandwidth

@0x9eb32e19f86ee174;

using Cxx = import "/capnp/c++.capnp";
$Cxx.namespace("tradecaptain::hft");

# Order side enumeration
enum Side {
  buy @0;
  sell @1;
}

# Order type enumeration
enum OrderType {
  market @0;
  limit @1;
  stop @2;
  stopLimit @3;
}

# Order status enumeration
enum OrderStatus {
  new @0;
  partialFill @1;
  filled @2;
  cancelled @3;
  rejected @4;
}

# Ultra-fast order message (32 bytes when packed)
struct OrderMessage {
  orderId @0 :UInt64;          # 8 bytes - unique order identifier
  symbol @1 :UInt64;           # 8 bytes - symbol as packed uint64
  side @2 :Side;               # 1 byte
  orderType @3 :OrderType;     # 1 byte
  price @4 :UInt64;            # 8 bytes - price in fixed-point (multiply by 1e-6)
  quantity @5 :UInt64;         # 8 bytes - quantity in fixed-point
  timestamp @6 :UInt64;        # 8 bytes - nanoseconds since epoch
}

# Market data tick (24 bytes when packed)
struct MarketDataTick {
  symbol @0 :UInt64;           # 8 bytes - symbol as packed uint64
  price @1 :UInt64;            # 8 bytes - price in fixed-point
  volume @2 :UInt64;           # 8 bytes - volume
  timestamp @3 :UInt64;        # 8 bytes - nanoseconds since epoch
}

# Level 2 order book entry (32 bytes when packed)
struct BookEntry {
  symbol @0 :UInt64;           # 8 bytes
  side @1 :Side;               # 1 byte
  price @2 :UInt64;            # 8 bytes
  size @3 :UInt64;             # 8 bytes
  orderCount @4 :UInt32;       # 4 bytes
  timestamp @5 :UInt64;        # 8 bytes
}

# Trade execution (40 bytes when packed)
struct TradeExecution {
  tradeId @0 :UInt64;          # 8 bytes
  orderId @1 :UInt64;          # 8 bytes
  symbol @2 :UInt64;           # 8 bytes
  side @3 :Side;               # 1 byte
  price @4 :UInt64;            # 8 bytes
  quantity @5 :UInt64;         # 8 bytes
  timestamp @6 :UInt64;        # 8 bytes
}

# Risk check result (16 bytes when packed)
struct RiskCheckResult {
  orderId @0 :UInt64;          # 8 bytes
  approved @1 :Bool;           # 1 byte
  reason @2 :UInt8;            # 1 byte - reason code
  maxQuantity @3 :UInt64;      # 8 bytes - max allowed quantity
}

# Portfolio position update (32 bytes when packed)
struct PositionUpdate {
  portfolioId @0 :UInt64;      # 8 bytes
  symbol @1 :UInt64;           # 8 bytes
  quantity @2 :Int64;          # 8 bytes - signed for long/short
  avgCost @3 :UInt64;          # 8 bytes
  timestamp @4 :UInt64;        # 8 bytes
}

# Message batch for efficient network transmission
struct MessageBatch {
  sequence @0 :UInt64;         # 8 bytes - sequence number
  timestamp @1 :UInt64;        # 8 bytes - batch timestamp
  orders @2 :List(OrderMessage);
  marketData @3 :List(MarketDataTick);
  trades @4 :List(TradeExecution);
  positions @5 :List(PositionUpdate);
}

# Session-level message wrapper
struct SessionMessage {
  sessionId @0 :UInt64;        # 8 bytes
  sequence @1 :UInt64;         # 8 bytes
  timestamp @2 :UInt64;        # 8 bytes

  union {
    order @3 :OrderMessage;
    marketData @4 :MarketDataTick;
    bookEntry @5 :BookEntry;
    trade @6 :TradeExecution;
    riskCheck @7 :RiskCheckResult;
    position @8 :PositionUpdate;
    batch @9 :MessageBatch;
  }
}