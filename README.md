# TradeCaptain

A modern, open-source financial data terminal built with Go, Rust, and React. This project provides real-time market data, portfolio management, technical analysis, and news aggregation using free and open-source APIs.

## 👨‍💻 Author
**Tarun Thakur**
Email: thakur[dot]cs[dot]tarun[at]gmail[dot]com

## 🏗️ Architecture

### System Overview

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

### Services

- **Data Collector (Go)**: Fetches real-time market data from various APIs
- **API Gateway (Go)**: RESTful API and WebSocket server for client communication
- **Calculation Engine (Rust)**: High-performance financial calculations and analytics
- **Frontend (React/TypeScript)**: Modern web interface with real-time charts
- **Infrastructure**: PostgreSQL, TimescaleDB, Redis, Kafka for data persistence and streaming

## 🚀 Features

### Phase 1 (MVP - Free Data Sources)
- [x] Real-time stock quotes (15-min delay via Yahoo Finance/Alpha Vantage)
- [x] Interactive candlestick charts with technical indicators
- [x] Portfolio tracking and P&L calculation
- [x] Financial news aggregation
- [x] Economic data dashboard (FRED API)
- [x] Stock screener with basic filters
- [x] User authentication and watchlists
- [x] WebSocket real-time updates

### Phase 2 (Enhanced Features)
- [ ] Mobile app (React Native)
- [ ] Advanced technical analysis
- [ ] Social sentiment analysis
- [ ] Options pricing calculator
- [ ] Risk management tools
- [ ] Backtesting framework

### Phase 3 (Premium Platform)
- [ ] Real-time premium data feeds
- [ ] Trading integration
- [ ] Institutional features
- [ ] Custom algorithmic strategies

## 🛠️ Technology Stack

### Backend
- **Go 1.21+**: Data collection, API gateway, concurrent processing
- **Rust**: High-performance financial calculations (Black-Scholes, VaR, etc.)
- **PostgreSQL**: Primary database for user data and market data
- **TimescaleDB**: Time-series data for historical prices
- **Redis**: Caching and session management
- **Kafka**: Real-time data streaming between services

### Frontend
- **React 18**: Modern UI framework
- **TypeScript**: Type-safe development
- **Vite**: Fast build tool and development server
- **TailwindCSS**: Utility-first styling
- **Recharts**: Financial charting library
- **Zustand**: Lightweight state management
- **React Query**: Data fetching and caching

### Infrastructure
- **Docker & Docker Compose**: Containerization
- **Nginx**: Reverse proxy and load balancing
- **Grafana**: Monitoring and analytics dashboards
- **Prometheus**: Metrics collection

## 📦 Quick Start

### Prerequisites
- Docker and Docker Compose
- Go 1.21+
- Rust 1.70+
- Node.js 18+

### 1. Clone and Setup
```bash
git clone <repository-url>
cd tradecaptain
make dev-setup
```

### 2. Configure API Keys
Edit `.env` file with your API keys:
```bash
# Free API Keys (register with providers)
ALPHA_VANTAGE_API_KEY=your_key_here
IEX_CLOUD_API_KEY=your_key_here
NEWS_API_KEY=your_key_here
FRED_API_KEY=your_key_here
```

### 3. Start Services
```bash
# Start all services with Docker
make run

# Or run locally for development
make run-local
```

### 4. Access the Application
- **Frontend**: http://localhost:3000
- **API Gateway**: http://localhost:8080
- **API Documentation**: http://localhost:8080/swagger/index.html
- **Grafana Dashboard**: http://localhost:3001 (admin/admin)

## 🔧 Development

### Running Individual Services

```bash
# Data collector
make run-data-collector

# API gateway
make run-api-gateway

# Calculation engine
make run-calc-engine

# Frontend
make run-frontend
```

### Testing

```bash
# Run all tests
make test

# Test individual services
make test-go
make test-rust
make test-frontend
```

### Linting and Formatting

```bash
# Lint all services
make lint

# Format code
make lint-go
make lint-rust
make lint-frontend
```

## 📊 Data Sources

### Current (Free/Open Source)
- **Yahoo Finance**: Basic stock quotes (15-min delay)
- **Alpha Vantage**: Real-time quotes (limited requests)
- **IEX Cloud**: US market data (freemium)
- **FRED API**: Economic indicators (Federal Reserve)
- **News APIs**: Financial news aggregation
- **CoinGecko**: Cryptocurrency data

### Future Premium Upgrades
- Refinitiv (formerly Thomson Reuters)
- S&P Capital IQ
- FactSet
- Direct exchange feeds

## 🏃‍♂️ Performance Expectations

With the current architecture:
- **Sub-millisecond** calculation latency (Rust engine)
- **10,000+ concurrent users** on single server
- **Real-time processing** of market data streams
- **Microsecond-level** WebSocket updates

## 📁 Project Structure

```
tradecaptain/
├── services/
│   ├── data-collector/     # Go service for data collection
│   ├── api-gateway/        # Go API server with WebSocket
│   └── calculation-engine/ # Rust high-performance calculations
├── frontend/               # React/TypeScript web app
├── infrastructure/         # Docker, Nginx, monitoring configs
├── database/              # Database schemas and migrations
├── docs/                  # Documentation
└── scripts/              # Utility scripts
```

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📜 License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.

## ⚠️ Disclaimer

This software is for educational and research purposes only. It is not intended for actual trading or investment decisions. Always consult with a qualified financial advisor before making investment decisions.

## 🙏 Acknowledgments

- [TradingView](https://www.tradingview.com/) for charting inspiration
- Bloomberg Terminal for feature reference
- Open source financial libraries: QuantLib, TA-Lib, etc.
- Free data providers: Yahoo Finance, Alpha Vantage, FRED, IEX Cloud

---

**Built with ❤️ for the open-source financial community**