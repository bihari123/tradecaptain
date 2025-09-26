# Getting Started - TradeCaptain

## 👨‍💻 **Project Author**
**Tarun Thakur**
Email: thakur[dot]cs[dot]tarun[at]gmail[dot]com

## 🎯 **For New Team Members**

Welcome to the TradeCaptain project! This guide will help you understand the system architecture, set up your development environment, and start contributing effectively.

## 📋 **Prerequisites**

### **Required Software**
```bash
# Development Tools
- Docker & Docker Compose (latest)
- Go 1.21+
- Rust 1.70+
- Node.js 18+
- PostgreSQL 15+ (for local development)
- Redis 7+ (for local development)

# Recommended IDEs
- VS Code with Go, Rust, and TypeScript extensions
- GoLand (JetBrains)
- RustRover or IntelliJ IDEA with Rust plugin
```

### **API Keys (Free Tier)**
Register for free API keys from these providers:
```bash
# Market Data APIs
ALPHA_VANTAGE_API_KEY    # https://www.alphavantage.co/support/#api-key
IEX_CLOUD_API_KEY        # https://iexcloud.io/
NEWS_API_KEY             # https://newsapi.org/
FRED_API_KEY             # https://fred.stlouisfed.org/docs/api/api_key.html
```

## 🏗️ **System Architecture Overview**

Our system follows a **microservices architecture** with **multi-language implementation**:

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   Frontend      │    │   API Gateway    │    │ Data Collector  │
│   (React/TS)    │◄──►│   (Go/Gin)       │◄──►│   (Go)          │
└─────────────────┘    └──────────────────┘    └─────────────────┘
                                │                        │
                                ▼                        ▼
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│ Calculation     │    │   PostgreSQL     │    │     Kafka       │
│ Engine (Rust)   │    │   + TimescaleDB  │    │   (Streaming)   │
└─────────────────┘    └──────────────────┘    └─────────────────┘
         │                       │                        │
         └───────────────────────┼────────────────────────┘
                                 ▼
                       ┌──────────────────┐
                       │      Redis       │
                       │     (Cache)      │
                       └──────────────────┘
```

### **Why Multi-Language Architecture?**

| Language | Use Case | Reasoning |
|----------|----------|-----------|
| **Go** | API Gateway, Data Collection | Excellent concurrency, great for I/O operations |
| **Rust** | Financial Calculations | Maximum performance, memory safety for critical calculations |
| **TypeScript** | Frontend | Type safety, modern React development |

## 🚀 **Quick Start (5 minutes)**

### **1. Clone and Setup**
```bash
git clone <repository-url>
cd tradecaptain

# Copy environment template
cp .env.example .env

# Edit .env with your API keys
nano .env
```

### **2. Start Development Environment**
```bash
# Start all services with Docker
make run

# OR start infrastructure only and run services locally
make run-local
```

### **3. Verify Setup**
```bash
# Check all services are running
curl http://localhost:8080/health

# Check frontend
open http://localhost:3000

# Check API documentation
open http://localhost:8080/swagger/index.html
```

## 📁 **Project Structure**

```
tradecaptain/
├── services/
│   ├── data-collector/     # Go - Market data collection
│   ├── api-gateway/        # Go - REST API and WebSocket
│   └── calculation-engine/ # Rust - Financial calculations
├── frontend/               # React/TypeScript UI
├── infrastructure/         # Docker, Nginx, monitoring
├── database/              # SQL schemas and migrations
├── docs/                  # Documentation
├── scripts/              # Utility scripts
└── tests/                # Integration tests
```

## 🔄 **Development Workflow**

### **Branch Strategy**
```bash
main                    # Production ready code
├── develop            # Integration branch
├── feature/my-feature # Feature development
├── bugfix/issue-123   # Bug fixes
└── hotfix/urgent-fix  # Emergency fixes
```

### **Daily Development**
```bash
# 1. Start development environment
make run-local

# 2. Run tests before coding
make test

# 3. Implement features with tests
make test-go          # Test Go services
make test-rust        # Test Rust calculations
make test-frontend    # Test React components

# 4. Run linting
make lint

# 5. Create commit
git add .
git commit -m "feat: implement Black-Scholes calculator"

# 6. Push and create PR
git push origin feature/black-scholes
```

## 🎓 **Learning Path for New Developers**

### **Week 1: Understanding the Domain**
1. **Read Documentation**
   - [System Architecture](./ARCHITECTURE.md)
   - [Design Patterns](./DESIGN_PATTERNS.md)
   - [API Documentation](./API.md)

2. **Explore Codebase**
   - Start with `services/calculation-engine/src/financial.rs`
   - Review `services/api-gateway/internal/handlers/`
   - Examine frontend components in `frontend/src/components/`

3. **Run Existing Tests**
   ```bash
   # Understanding through tests
   make test

   # Look at test files to understand expected behavior
   find . -name "*_test.go" -o -name "*_test.rs" -o -name "*.test.ts"
   ```

### **Week 2: First Contributions**
1. **Pick a TODO Item**
   - Search for `TODO` comments in the codebase
   - Start with simple calculations or utility functions
   - Example: Implement `simple_moving_average` in technical.rs

2. **Implementation Pattern**
   ```rust
   // Example: Implementing SMA in Rust
   pub fn simple_moving_average(&self, prices: &[f64], period: usize) -> Result<f64> {
       if prices.len() < period {
           return Err(anyhow::anyhow!("Insufficient data for SMA"));
       }

       let sum: f64 = prices.iter().rev().take(period).sum();
       Ok(sum / period as f64)
   }

   #[cfg(test)]
   mod tests {
       use super::*;

       #[test]
       fn test_simple_moving_average() {
           let calc = TechnicalIndicators::new();
           let prices = vec![1.0, 2.0, 3.0, 4.0, 5.0];
           let result = calc.simple_moving_average(&prices, 3).unwrap();
           assert_eq!(result, 4.0); // (3+4+5)/3
       }
   }
   ```

### **Week 3-4: Advanced Features**
1. **API Integration**
   - Implement external API clients
   - Add error handling and retry logic
   - Write integration tests

2. **Frontend Components**
   - Create React components with TypeScript
   - Implement real-time data updates
   - Add responsive design

## 🧪 **Testing Philosophy**

### **Test Pyramid**
```
    E2E Tests (Few)
       /\
      /  \
     /____\
    Integration Tests (Some)
           /\
          /  \
         /____\
        Unit Tests (Many)
```

### **Testing Commands**
```bash
# Unit Tests (Fast - Run Frequently)
make test-go          # Go unit tests
make test-rust        # Rust unit tests
make test-frontend    # React component tests

# Integration Tests (Slower - Run Before Commits)
make test-integration

# End-to-End Tests (Slowest - Run Before Releases)
make test-e2e

# Performance Tests
make benchmark
```

### **Writing Good Tests**
```go
// Example: Go unit test
func TestMarketData_SaveAndRetrieve(t *testing.T) {
    // Arrange
    db := setupTestDB(t)
    ctx := context.Background()

    marketData := &models.MarketData{
        Symbol: "AAPL",
        Price:  150.25,
        // ... other fields
    }

    // Act
    err := db.SaveMarketData(ctx, marketData)

    // Assert
    require.NoError(t, err)

    retrieved, err := db.GetLatestMarketData(ctx, []string{"AAPL"})
    require.NoError(t, err)
    assert.Equal(t, marketData.Symbol, retrieved[0].Symbol)
}
```

## 📊 **Monitoring and Debugging**

### **Local Development Monitoring**
```bash
# View service logs
make logs

# Monitor specific service
make logs-data-collector
make logs-api-gateway

# Check database
psql postgres://bloomberg_user:bloomberg_pass@localhost:5432/bloomberg_terminal

# Check Redis
redis-cli -h localhost -p 6379

# Monitor Kafka
kafka-console-consumer --bootstrap-server localhost:9092 --topic market-data
```

### **Performance Monitoring**
```bash
# Go services profiling
go tool pprof http://localhost:8080/debug/pprof/profile

# Rust benchmarking
cd services/calculation-engine
cargo bench

# Database query analysis
EXPLAIN ANALYZE SELECT * FROM market_data WHERE symbol = 'AAPL';
```

## 🎯 **Common Tasks for New Developers**

### **1. Adding a New Financial Calculation**
```rust
// 1. Add function signature in calculation-engine/src/financial.rs
pub fn new_calculation(&self, param1: f64, param2: f64) -> Result<f64> {
    // TODO: Implement calculation
    panic!("TODO: Implement new calculation")
}

// 2. Write test first
#[test]
fn test_new_calculation() {
    let calc = FinancialCalculator::new();
    let result = calc.new_calculation(100.0, 0.05).unwrap();
    assert_eq!(result, expected_value);
}

// 3. Implement the calculation
// 4. Add API endpoint in api-gateway
// 5. Add frontend component if needed
```

### **2. Adding a New API Endpoint**
```go
// 1. Add handler in api-gateway/internal/handlers/
func (h *Handler) NewEndpoint(c *gin.Context) {
    // Extract parameters
    // Call business logic
    // Return response
}

// 2. Add route in main.go
router.GET("/api/new-endpoint", handler.NewEndpoint)

// 3. Add Swagger documentation
// @Summary Description
// @Router /api/new-endpoint [get]

// 4. Write integration test
```

### **3. Adding External Data Source**
```go
// 1. Create client in data-collector/internal/collector/
type NewAPIClient struct {
    httpClient *http.Client
    apiKey     string
}

// 2. Implement interface methods
func (c *NewAPIClient) GetData(symbol string) (*MarketData, error) {
    // Make API request
    // Parse response
    // Return structured data
}

// 3. Add to collector orchestrator
// 4. Add configuration
// 5. Write tests with mock responses
```

## 🔧 **Troubleshooting Common Issues**

### **Docker Issues**
```bash
# Clean docker environment
docker-compose down -v
docker system prune -f

# Rebuild services
make build

# Check logs for specific service
docker-compose logs -f api-gateway
```

### **Database Issues**
```bash
# Reset database
make db-reset

# Run migrations manually
docker-compose exec postgres psql -U bloomberg_user -d bloomberg_terminal -f /docker-entrypoint-initdb.d/init.sql
```

### **Go Build Issues**
```bash
# Update dependencies
go mod tidy
go mod download

# Clear module cache
go clean -modcache
```

### **Rust Build Issues**
```bash
# Update Rust
rustup update

# Clean build artifacts
cargo clean

# Update dependencies
cargo update
```

## 📚 **Additional Resources**

### **Documentation**
- [Architecture Deep Dive](./ARCHITECTURE.md)
- [API Reference](./API.md)
- [Design Patterns Used](./DESIGN_PATTERNS.md)
- [Deployment Guide](./DEPLOYMENT.md)

### **External Learning Resources**
- [Go Best Practices](https://golang.org/doc/effective_go.html)
- [Rust Book](https://doc.rust-lang.org/book/)
- [React TypeScript Guide](https://react-typescript-cheatsheet.netlify.app/)
- [Financial Calculations Reference](https://en.wikipedia.org/wiki/Black%E2%80%93Scholes_model)

### **Community and Support**
- **Slack Channel**: #tradecaptain-dev
- **Code Reviews**: Required for all PRs
- **Weekly Standups**: Wednesdays 10 AM
- **Architecture Reviews**: Fridays 2 PM

## 🎉 **Your First Week Goals**

- [ ] Set up complete development environment
- [ ] Run all existing tests successfully
- [ ] Understand system architecture
- [ ] Complete one small TODO item
- [ ] Submit your first pull request
- [ ] Join team communication channels

**Welcome to the team! 🚀**