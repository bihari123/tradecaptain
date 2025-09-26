# Ultra-High-Performance Design Patterns and Best Practices

## 🏗️ **Expert-Level Architectural Patterns**

### **1. Ultra-Performance Microservices Architecture**

**Pattern**: Decompose into microsecond-optimized, independent services with three-phase performance engineering.

**Implementation**:
```go
// Each service optimized for institutional-grade performance
services/
├── data-collector/     # Go + Aeron messaging + BigCache + BadgerDB WAL
├── api-gateway/        # Go + io_uring + FlatBuffers + ClickHouse
└── calculation-engine/ # Rust + NUMA + memory-mapped + cache-aligned
```

**Ultra-Performance Benefits**:
- **<100μs** inter-service communication via Aeron
- **Zero-GC** performance with embedded caches
- **NUMA-aware** deployment for multi-socket systems
- **Memory-mapped** persistence for nanosecond writes
- **Lock-free** data structures for maximum concurrency

**Best Practices**:
```go
// Service communication through well-defined APIs
type MarketDataService interface {
    GetQuote(ctx context.Context, symbol string) (*MarketData, error)
    GetHistory(ctx context.Context, symbol string, period Period) ([]*MarketData, error)
}
```

### **2. Ultra-Low-Latency Event-Driven Architecture**

**Pattern**: Services communicate through microsecond-latency messaging with zero-copy serialization.

**Implementation**:
```go
// Aeron ultra-low latency messaging
func (p *AeronProducer) PublishMarketData(ctx context.Context, data *MarketData) error {
    // Cap'n Proto zero-copy serialization
    msg := capnp.NewMessage(capnp.SingleSegment(nil))
    event, _ := NewRootMarketDataEvent(msg.Segment())

    event.SetSymbol(data.Symbol)
    event.SetPrice(data.Price)
    event.SetTimestamp(data.Timestamp)

    // <100μs end-to-end delivery
    return p.publication.Offer(msg.Bytes(), 0, len(msg.Bytes()))
}

// Event consumption
func (c *Consumer) handleMarketDataEvent(event MarketDataEvent) error {
    // Update caches, trigger calculations, notify clients
    return c.processEvent(event)
}
```

**Benefits**:
- Loose coupling between services
- Real-time data propagation
- Event sourcing capability
- Replay and recovery mechanisms

### **3. CQRS (Command Query Responsibility Segregation)**

**Pattern**: Separate read and write operations for better performance.

**Implementation**:
```go
// Command side - Data ingestion and updates
type MarketDataWriter interface {
    SaveMarketData(ctx context.Context, data *MarketData) error
    UpdateBatch(ctx context.Context, batch []*MarketData) error
}

// Query side - Optimized for reads
type MarketDataReader interface {
    GetQuote(ctx context.Context, symbol string) (*MarketData, error)
    GetHistoricalData(ctx context.Context, params QueryParams) ([]*MarketData, error)
}
```

**Benefits**:
- Optimized data models for reads vs writes
- Independent scaling of read/write operations
- Better performance for complex queries

---

## 🎯 **Design Patterns by Service**

### **Data Collector Service (Go)**

#### **1. Repository Pattern**
```go
// Abstract storage operations
type MarketDataRepository interface {
    Save(ctx context.Context, data *MarketData) error
    FindBySymbol(ctx context.Context, symbol string) ([]*MarketData, error)
    FindByTimeRange(ctx context.Context, from, to time.Time) ([]*MarketData, error)
}

// Concrete implementation
type PostgreSQLRepository struct {
    db *sql.DB
}

func (r *PostgreSQLRepository) Save(ctx context.Context, data *MarketData) error {
    query := `INSERT INTO market_data (...) VALUES (...) ON CONFLICT (...) DO UPDATE`
    _, err := r.db.ExecContext(ctx, query, data.Fields()...)
    return err
}
```

**Benefits**:
- Abstracts database operations
- Easy testing with mock repositories
- Database-agnostic business logic

#### **2. Factory Pattern**
```go
// API client factory
type APIClientFactory interface {
    CreateClient(provider string, config Config) (MarketDataClient, error)
}

type DefaultAPIClientFactory struct{}

func (f *DefaultAPIClientFactory) CreateClient(provider string, config Config) (MarketDataClient, error) {
    switch provider {
    case "yahoo":
        return NewYahooFinanceClient(config.APIKey), nil
    case "alphavantage":
        return NewAlphaVantageClient(config.APIKey), nil
    default:
        return nil, fmt.Errorf("unknown provider: %s", provider)
    }
}
```

**Benefits**:
- Easy addition of new data providers
- Centralized client creation logic
- Configuration management

#### **3. Circuit Breaker Pattern**
```go
type CircuitBreaker struct {
    failureThreshold int
    resetTimeout     time.Duration
    failureCount     int
    lastFailureTime  time.Time
    state           State
}

func (cb *CircuitBreaker) Call(fn func() error) error {
    if cb.state == Open {
        if time.Since(cb.lastFailureTime) > cb.resetTimeout {
            cb.state = HalfOpen
        } else {
            return ErrCircuitBreakerOpen
        }
    }

    err := fn()
    if err != nil {
        cb.recordFailure()
        return err
    }

    cb.recordSuccess()
    return nil
}
```

**Benefits**:
- Prevents cascade failures
- Automatic recovery
- System stability under load

#### **4. Rate Limiter Pattern**
```go
type TokenBucket struct {
    capacity    int
    tokens      int
    refillRate  time.Duration
    lastRefill  time.Time
    mutex       sync.Mutex
}

func (tb *TokenBucket) Allow() bool {
    tb.mutex.Lock()
    defer tb.mutex.Unlock()

    now := time.Now()
    tokensToAdd := int(now.Sub(tb.lastRefill) / tb.refillRate)

    tb.tokens = min(tb.capacity, tb.tokens+tokensToAdd)
    tb.lastRefill = now

    if tb.tokens > 0 {
        tb.tokens--
        return true
    }
    return false
}
```

**Benefits**:
- Respects API rate limits
- Prevents service degradation
- Fair resource allocation

### **Calculation Engine (Rust)**

#### **1. Strategy Pattern**
```rust
// Different calculation strategies
trait VaRStrategy {
    fn calculate(&self, returns: &[f64], confidence: f64) -> Result<f64>;
}

struct HistoricalSimulation;
impl VaRStrategy for HistoricalSimulation {
    fn calculate(&self, returns: &[f64], confidence: f64) -> Result<f64> {
        // Historical simulation implementation
    }
}

struct ParametricVaR;
impl VaRStrategy for ParametricVaR {
    fn calculate(&self, returns: &[f64], confidence: f64) -> Result<f64> {
        // Parametric VaR implementation
    }
}

// Context using strategy
struct RiskCalculator {
    strategy: Box<dyn VaRStrategy>,
}

impl RiskCalculator {
    fn calculate_var(&self, returns: &[f64], confidence: f64) -> Result<f64> {
        self.strategy.calculate(returns, confidence)
    }
}
```

**Benefits**:
- Multiple calculation methods
- Easy algorithm switching
- Extensible calculation framework

#### **2. Builder Pattern**
```rust
// Complex option pricing setup
pub struct OptionPriceBuilder {
    spot_price: Option<f64>,
    strike_price: Option<f64>,
    time_to_expiry: Option<f64>,
    risk_free_rate: Option<f64>,
    volatility: Option<f64>,
    dividend_yield: Option<f64>,
}

impl OptionPriceBuilder {
    pub fn new() -> Self {
        Self::default()
    }

    pub fn spot_price(mut self, price: f64) -> Self {
        self.spot_price = Some(price);
        self
    }

    pub fn strike_price(mut self, strike: f64) -> Self {
        self.strike_price = Some(strike);
        self
    }

    pub fn build(self) -> Result<OptionParameters> {
        Ok(OptionParameters {
            spot_price: self.spot_price.ok_or("spot_price required")?,
            strike_price: self.strike_price.ok_or("strike_price required")?,
            // ... validate all required fields
        })
    }
}

// Usage
let option = OptionPriceBuilder::new()
    .spot_price(100.0)
    .strike_price(105.0)
    .time_to_expiry(0.25)
    .risk_free_rate(0.05)
    .volatility(0.20)
    .build()?;
```

**Benefits**:
- Clear parameter validation
- Fluent interface
- Compile-time safety

#### **3. Template Method Pattern**
```rust
// Abstract calculation template
trait TechnicalIndicator {
    fn validate_input(&self, data: &[f64]) -> Result<()>;
    fn calculate_core(&self, data: &[f64]) -> Result<f64>;
    fn post_process(&self, result: f64) -> f64 { result }

    // Template method
    fn calculate(&self, data: &[f64]) -> Result<f64> {
        self.validate_input(data)?;
        let result = self.calculate_core(data)?;
        Ok(self.post_process(result))
    }
}

struct SimpleMovingAverage {
    period: usize,
}

impl TechnicalIndicator for SimpleMovingAverage {
    fn validate_input(&self, data: &[f64]) -> Result<()> {
        if data.len() < self.period {
            return Err(anyhow!("Insufficient data"));
        }
        Ok(())
    }

    fn calculate_core(&self, data: &[f64]) -> Result<f64> {
        let sum: f64 = data.iter().rev().take(self.period).sum();
        Ok(sum / self.period as f64)
    }
}
```

**Benefits**:
- Consistent calculation flow
- Reusable validation logic
- Extensible indicator framework

### **API Gateway (Go)**

#### **1. Middleware Chain Pattern**
```go
type Middleware func(http.HandlerFunc) http.HandlerFunc

// Authentication middleware
func AuthMiddleware(secret string) Middleware {
    return func(next http.HandlerFunc) http.HandlerFunc {
        return func(w http.ResponseWriter, r *http.Request) {
            token := extractToken(r)
            if !validateToken(token, secret) {
                http.Error(w, "Unauthorized", http.StatusUnauthorized)
                return
            }
            next(w, r)
        }
    }
}

// Rate limiting middleware
func RateLimitMiddleware(limiter *rate.Limiter) Middleware {
    return func(next http.HandlerFunc) http.HandlerFunc {
        return func(w http.ResponseWriter, r *http.Request) {
            if !limiter.Allow() {
                http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
                return
            }
            next(w, r)
        }
    }
}

// Chain middlewares
func ChainMiddlewares(handler http.HandlerFunc, middlewares ...Middleware) http.HandlerFunc {
    for i := len(middlewares) - 1; i >= 0; i-- {
        handler = middlewares[i](handler)
    }
    return handler
}
```

**Benefits**:
- Reusable cross-cutting concerns
- Composable request processing
- Clean separation of concerns

#### **2. Adapter Pattern**
```go
// External service interface
type ExternalCalculationService interface {
    CalculateRisk(data RiskData) (float64, error)
}

// Rust calculation service adapter
type RustCalculationAdapter struct {
    rustService *RustCalculationEngine
}

func (a *RustCalculationAdapter) CalculateRisk(data RiskData) (float64, error) {
    // Convert Go data structures to Rust-compatible format
    rustData := convertToRustFormat(data)

    // Call Rust service via FFI
    result, err := a.rustService.CalculateVaR(rustData)
    if err != nil {
        return 0, fmt.Errorf("rust calculation failed: %w", err)
    }

    return result, nil
}
```

**Benefits**:
- Integrates different languages/systems
- Maintains consistent interfaces
- Hides implementation complexity

#### **3. Observer Pattern (WebSocket)**
```go
type MarketDataObserver interface {
    OnMarketDataUpdate(data *MarketData)
    GetSubscriptions() []string
}

type WebSocketClient struct {
    conn          *websocket.Conn
    subscriptions []string
    sendChannel   chan []byte
}

func (c *WebSocketClient) OnMarketDataUpdate(data *MarketData) {
    if c.isSubscribed(data.Symbol) {
        message := formatMarketDataMessage(data)
        select {
        case c.sendChannel <- message:
        default:
            // Channel full, drop message or handle appropriately
        }
    }
}

type MarketDataPublisher struct {
    observers []MarketDataObserver
    mutex     sync.RWMutex
}

func (p *MarketDataPublisher) NotifyObservers(data *MarketData) {
    p.mutex.RLock()
    defer p.mutex.RUnlock()

    for _, observer := range p.observers {
        go observer.OnMarketDataUpdate(data) // Non-blocking
    }
}
```

**Benefits**:
- Real-time data distribution
- Decoupled publisher-subscriber
- Scalable notification system

---

## 🎨 **Frontend Patterns (React/TypeScript)**

### **1. Custom Hooks Pattern**
```typescript
// Market data hook
function useMarketData(symbol: string) {
    const [data, setData] = useState<MarketData | null>(null);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);

    useEffect(() => {
        const fetchData = async () => {
            try {
                setLoading(true);
                const response = await marketDataAPI.getQuote(symbol);
                setData(response.data);
                setError(null);
            } catch (err) {
                setError(err.message);
            } finally {
                setLoading(false);
            }
        };

        fetchData();

        // Set up WebSocket subscription
        const unsubscribe = marketDataService.subscribe(symbol, setData);
        return unsubscribe;
    }, [symbol]);

    return { data, loading, error };
}

// Usage in component
function StockQuote({ symbol }: { symbol: string }) {
    const { data, loading, error } = useMarketData(symbol);

    if (loading) return <LoadingSpinner />;
    if (error) return <ErrorMessage error={error} />;
    if (!data) return null;

    return <QuoteDisplay data={data} />;
}
```

### **2. State Management Pattern (Zustand)**
```typescript
interface MarketStore {
    quotes: Record<string, MarketData>;
    subscriptions: Set<string>;

    addQuote: (symbol: string, data: MarketData) => void;
    subscribe: (symbol: string) => void;
    unsubscribe: (symbol: string) => void;
    getQuote: (symbol: string) => MarketData | undefined;
}

const useMarketStore = create<MarketStore>((set, get) => ({
    quotes: {},
    subscriptions: new Set(),

    addQuote: (symbol, data) =>
        set(state => ({
            quotes: { ...state.quotes, [symbol]: data }
        })),

    subscribe: (symbol) => {
        set(state => ({
            subscriptions: new Set([...state.subscriptions, symbol])
        }));

        // Set up WebSocket subscription
        websocketService.subscribe(symbol);
    },

    unsubscribe: (symbol) => {
        set(state => {
            const newSubscriptions = new Set(state.subscriptions);
            newSubscriptions.delete(symbol);
            return { subscriptions: newSubscriptions };
        });

        websocketService.unsubscribe(symbol);
    },

    getQuote: (symbol) => get().quotes[symbol],
}));
```

---

## 🔧 **Error Handling Patterns**

### **1. Result Pattern (Rust-Style Error Handling)**

**Rust Implementation**:
```rust
// Custom error types
#[derive(thiserror::Error, Debug)]
pub enum CalculationError {
    #[error("Invalid input: {0}")]
    InvalidInput(String),
    #[error("Insufficient data: need at least {required}, got {actual}")]
    InsufficientData { required: usize, actual: usize },
    #[error("Numerical error: {0}")]
    NumericalError(String),
}

// Function returning Result
pub fn calculate_var(returns: &[f64], confidence: f64) -> Result<f64, CalculationError> {
    if returns.is_empty() {
        return Err(CalculationError::InsufficientData {
            required: 1,
            actual: 0
        });
    }

    if !(0.0..=1.0).contains(&confidence) {
        return Err(CalculationError::InvalidInput(
            "Confidence must be between 0 and 1".to_string()
        ));
    }

    // Calculation logic...
    Ok(calculated_var)
}
```

**Go Implementation**:
```go
// Custom error types
type CalculationError struct {
    Code    string
    Message string
    Details map[string]interface{}
}

func (e CalculationError) Error() string {
    return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Error constants
var (
    ErrInvalidInput      = "INVALID_INPUT"
    ErrInsufficientData  = "INSUFFICIENT_DATA"
    ErrCalculationFailed = "CALCULATION_FAILED"
)

// Function with proper error handling
func CalculateVaR(returns []float64, confidence float64) (float64, error) {
    if len(returns) == 0 {
        return 0, CalculationError{
            Code:    ErrInsufficientData,
            Message: "No return data provided",
            Details: map[string]interface{}{
                "required": 1,
                "actual":   0,
            },
        }
    }

    // Calculation logic...
    return result, nil
}
```

### **2. Circuit Breaker Error Handling**
```go
func (s *MarketDataService) GetQuote(ctx context.Context, symbol string) (*MarketData, error) {
    return s.circuitBreaker.Execute(func() (interface{}, error) {
        // Try cache first
        if cached, found := s.cache.Get(symbol); found {
            return cached, nil
        }

        // Try primary API
        data, err := s.primaryAPI.GetQuote(ctx, symbol)
        if err == nil {
            s.cache.Set(symbol, data)
            return data, nil
        }

        // Fallback to secondary API
        data, err = s.fallbackAPI.GetQuote(ctx, symbol)
        if err == nil {
            s.cache.Set(symbol, data)
            return data, nil
        }

        return nil, fmt.Errorf("all APIs failed: %w", err)
    })
}
```

---

## 📊 **Performance Patterns**

### **1. Object Pool Pattern**
```go
// Connection pool for database
type ConnectionPool struct {
    pool    sync.Pool
    factory func() (*sql.DB, error)
}

func NewConnectionPool(factory func() (*sql.DB, error)) *ConnectionPool {
    return &ConnectionPool{
        factory: factory,
        pool: sync.Pool{
            New: func() interface{} {
                conn, _ := factory()
                return conn
            },
        },
    }
}

func (p *ConnectionPool) Get() *sql.DB {
    return p.pool.Get().(*sql.DB)
}

func (p *ConnectionPool) Put(conn *sql.DB) {
    p.pool.Put(conn)
}
```

### **2. Batch Processing Pattern**
```go
type BatchProcessor struct {
    batchSize int
    timeout   time.Duration
    processor func([]*MarketData) error
    buffer    []*MarketData
    timer     *time.Timer
    mutex     sync.Mutex
}

func (bp *BatchProcessor) Add(data *MarketData) {
    bp.mutex.Lock()
    defer bp.mutex.Unlock()

    bp.buffer = append(bp.buffer, data)

    if len(bp.buffer) == 1 {
        bp.timer = time.AfterFunc(bp.timeout, bp.flush)
    }

    if len(bp.buffer) >= bp.batchSize {
        bp.flushLocked()
    }
}

func (bp *BatchProcessor) flushLocked() {
    if len(bp.buffer) == 0 {
        return
    }

    if bp.timer != nil {
        bp.timer.Stop()
    }

    batch := make([]*MarketData, len(bp.buffer))
    copy(batch, bp.buffer)
    bp.buffer = bp.buffer[:0]

    go bp.processor(batch)
}
```

---

## 🎯 **Best Practices Summary**

### **Code Organization**
- **Package by feature, not by layer**
- **Clear separation of concerns**
- **Dependency injection for testability**
- **Interface segregation principle**

### **Error Handling**
- **Always handle errors explicitly**
- **Use custom error types for different error categories**
- **Implement proper error recovery mechanisms**
- **Log errors with sufficient context**

### **Performance**
- **Use connection pooling for databases**
- **Implement caching at appropriate layers**
- **Batch operations when possible**
- **Profile and measure performance continuously**

### **Testing**
- **Write tests first (TDD)**
- **Test behavior, not implementation**
- **Use dependency injection for mockability**
- **Maintain high test coverage (>80%)**

### **Concurrency**
- **Use channels for communication in Go**
- **Prefer immutable data structures**
- **Handle context cancellation properly**
- **Avoid shared mutable state**

### **Monitoring and Observability**
- **Structured logging with context**
- **Metrics for all business operations**
- **Distributed tracing for request flows**
- **Health checks for all services**

These patterns and practices ensure our Bloomberg Terminal Alternative is maintainable, scalable, and robust for production use.