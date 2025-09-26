#!/bin/bash

# TradeCaptain - Test Deployment Script
# Author: Tarun Thakur (thakur[dot]cs[dot]tarun[at]gmail[dot]com)
# This script sets up a complete testing environment with all services

set -e  # Exit on any error

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
COMPOSE_FILE="docker-compose.test.yml"
NETWORK_NAME="tradecaptain_test_network"
POSTGRES_READY_TIMEOUT=60
KAFKA_READY_TIMEOUT=60
TEST_RESULTS_DIR="./test-results"

# Functions
log() {
    echo -e "${BLUE}[$(date +'%Y-%m-%d %H:%M:%S')]${NC} $1"
}

success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

cleanup() {
    log "Cleaning up test environment..."
    docker-compose -f $COMPOSE_FILE down -v --remove-orphans 2>/dev/null || true
    docker network rm $NETWORK_NAME 2>/dev/null || true
    success "Cleanup completed"
}

wait_for_service() {
    local service_name=$1
    local port=$2
    local timeout=${3:-30}

    log "Waiting for $service_name to be ready on port $port..."

    for i in $(seq 1 $timeout); do
        if docker run --rm --network $NETWORK_NAME alpine:latest sh -c "nc -z $service_name $port" 2>/dev/null; then
            success "$service_name is ready!"
            return 0
        fi
        sleep 1
    done

    error "$service_name failed to start within $timeout seconds"
    return 1
}

wait_for_postgres() {
    log "Waiting for PostgreSQL to be ready..."

    for i in $(seq 1 $POSTGRES_READY_TIMEOUT); do
        if docker-compose -f $COMPOSE_FILE exec -T postgres pg_isready -U tradecaptain_user -d tradecaptain_test 2>/dev/null; then
            success "PostgreSQL is ready!"
            return 0
        fi
        sleep 1
    done

    error "PostgreSQL failed to start within $POSTGRES_READY_TIMEOUT seconds"
    return 1
}

wait_for_kafka() {
    log "Waiting for Kafka to be ready..."

    for i in $(seq 1 $KAFKA_READY_TIMEOUT); do
        if docker-compose -f $COMPOSE_FILE exec -T kafka kafka-topics --bootstrap-server localhost:9092 --list 2>/dev/null; then
            success "Kafka is ready!"
            return 0
        fi
        sleep 2
    done

    error "Kafka failed to start within $KAFKA_READY_TIMEOUT seconds"
    return 1
}

setup_test_environment() {
    log "Setting up test environment..."

    # Create test results directory
    mkdir -p $TEST_RESULTS_DIR

    # Copy test environment file
    if [ ! -f ".env.test" ]; then
        log "Creating test environment file..."
        cat > .env.test << EOF
# Test Environment Configuration
DATABASE_URL=postgres://tradecaptain_user:tradecaptain_pass@postgres:5432/tradecaptain_test?sslmode=disable
REDIS_URL=redis://redis:6379/1
KAFKA_BOOTSTRAP_SERVERS=kafka:9092

# Test API Keys (using demo/mock keys)
ALPHA_VANTAGE_API_KEY=demo
IEX_CLOUD_API_KEY=demo
NEWS_API_KEY=demo
FRED_API_KEY=demo

# Test Configuration
ENVIRONMENT=test
LOG_LEVEL=debug
RATE_LIMIT_PER_SECOND=1000

# Test Symbols
STOCK_SYMBOLS=AAPL,GOOGL,MSFT
CRYPTO_SYMBOLS=BTC,ETH
EOF
    fi

    success "Test environment configured"
}

create_test_compose_file() {
    log "Creating test-specific docker-compose file..."

    cat > $COMPOSE_FILE << 'EOF'
version: '3.8'

services:
  # Test Database
  postgres:
    image: postgres:15
    container_name: tradecaptain_postgres_test
    environment:
      POSTGRES_DB: tradecaptain_test
      POSTGRES_USER: tradecaptain_user
      POSTGRES_PASSWORD: tradecaptain_pass
    ports:
      - "5433:5432"
    volumes:
      - postgres_test_data:/var/lib/postgresql/data
      - ./database/test_schema.sql:/docker-entrypoint-initdb.d/init.sql
    networks:
      - tradecaptain_test_network
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U tradecaptain_user -d tradecaptain_test"]
      interval: 10s
      timeout: 5s
      retries: 5

  # Test Redis
  redis:
    image: redis:7-alpine
    container_name: tradecaptain_redis_test
    ports:
      - "6380:6379"
    volumes:
      - redis_test_data:/data
    networks:
      - tradecaptain_test_network
    command: redis-server --appendonly yes --databases 16
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 3s
      retries: 5

  # Test Kafka Setup
  zookeeper:
    image: confluentinc/cp-zookeeper:7.4.0
    container_name: tradecaptain_zookeeper_test
    environment:
      ZOOKEEPER_CLIENT_PORT: 2181
      ZOOKEEPER_TICK_TIME: 2000
    networks:
      - tradecaptain_test_network

  kafka:
    image: confluentinc/cp-kafka:7.4.0
    container_name: tradecaptain_kafka_test
    depends_on:
      - zookeeper
    ports:
      - "9093:9092"
    environment:
      KAFKA_BROKER_ID: 1
      KAFKA_ZOOKEEPER_CONNECT: zookeeper:2181
      KAFKA_ADVERTISED_LISTENERS: PLAINTEXT://kafka:9092
      KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR: 1
      KAFKA_TRANSACTION_STATE_LOG_MIN_ISR: 1
      KAFKA_TRANSACTION_STATE_LOG_REPLICATION_FACTOR: 1
      KAFKA_AUTO_CREATE_TOPICS_ENABLE: true
    networks:
      - tradecaptain_test_network
    healthcheck:
      test: ["CMD", "kafka-topics", "--bootstrap-server", "localhost:9092", "--list"]
      interval: 10s
      timeout: 10s
      retries: 5

  # Data Collector Service (Test Mode)
  data-collector:
    build:
      context: ./services/data-collector
      dockerfile: Dockerfile.test
    container_name: tradecaptain_data_collector_test
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy
      kafka:
        condition: service_healthy
    environment:
      - DATABASE_URL=postgres://tradecaptain_user:tradecaptain_pass@postgres:5432/tradecaptain_test?sslmode=disable
      - REDIS_URL=redis://redis:6379/1
      - KAFKA_BOOTSTRAP_SERVERS=kafka:9092
      - ENVIRONMENT=test
    env_file:
      - .env.test
    networks:
      - tradecaptain_test_network
    volumes:
      - ./test-results:/app/test-results

  # Calculation Engine (Test Mode)
  calculation-engine:
    build:
      context: ./services/calculation-engine
      dockerfile: Dockerfile.test
    container_name: tradecaptain_calc_engine_test
    depends_on:
      redis:
        condition: service_healthy
    environment:
      - REDIS_URL=redis://redis:6379/1
      - ENVIRONMENT=test
    networks:
      - tradecaptain_test_network
    volumes:
      - ./test-results:/app/test-results

  # API Gateway (Test Mode)
  api-gateway:
    build:
      context: ./services/api-gateway
      dockerfile: Dockerfile.test
    container_name: tradecaptain_api_gateway_test
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy
      kafka:
        condition: service_healthy
    ports:
      - "8081:8080"
    environment:
      - DATABASE_URL=postgres://tradecaptain_user:tradecaptain_pass@postgres:5432/tradecaptain_test?sslmode=disable
      - REDIS_URL=redis://redis:6379/1
      - KAFKA_BOOTSTRAP_SERVERS=kafka:9092
      - JWT_SECRET=test-jwt-secret
      - PORT=8080
      - ENVIRONMENT=test
    env_file:
      - .env.test
    networks:
      - tradecaptain_test_network
    volumes:
      - ./test-results:/app/test-results

  # Test Runner Service
  test-runner:
    build:
      context: ./tests
      dockerfile: Dockerfile
    container_name: tradecaptain_test_runner
    depends_on:
      - api-gateway
      - data-collector
      - calculation-engine
    environment:
      - API_BASE_URL=http://api-gateway:8080
      - DATABASE_URL=postgres://tradecaptain_user:tradecaptain_pass@postgres:5432/tradecaptain_test?sslmode=disable
      - REDIS_URL=redis://redis:6379/1
    networks:
      - tradecaptain_test_network
    volumes:
      - ./test-results:/app/results
      - ./tests:/app/tests
    command: ["python", "-m", "pytest", "-v", "--junitxml=/app/results/integration-tests.xml"]

volumes:
  postgres_test_data:
  redis_test_data:

networks:
  tradecaptain_test_network:
    driver: bridge
EOF

    success "Test compose file created"
}

run_unit_tests() {
    log "Running unit tests for all services..."

    # Go services unit tests
    log "Running Go unit tests..."
    cd services/data-collector
    go test -v -race -coverprofile=../../test-results/data-collector-coverage.out ./... > ../../test-results/data-collector-tests.log 2>&1 || true
    cd ../api-gateway
    go test -v -race -coverprofile=../../test-results/api-gateway-coverage.out ./... > ../../test-results/api-gateway-tests.log 2>&1 || true
    cd ../..

    # Rust service unit tests
    log "Running Rust unit tests..."
    cd services/calculation-engine
    cargo test --release -- --nocapture > ../../test-results/calculation-engine-tests.log 2>&1 || true
    cd ../..

    # Frontend unit tests (if applicable)
    if [ -d "frontend" ]; then
        log "Running frontend unit tests..."
        cd frontend
        npm test -- --coverage --watchAll=false > ../test-results/frontend-tests.log 2>&1 || true
        cd ..
    fi

    success "Unit tests completed"
}

run_integration_tests() {
    log "Running integration tests..."

    # Start services
    docker-compose -f $COMPOSE_FILE up -d

    # Wait for all services to be healthy
    wait_for_postgres
    wait_for_service redis 6379
    wait_for_kafka
    wait_for_service api-gateway 8080

    # Run integration test suite
    log "Executing integration test suite..."
    docker-compose -f $COMPOSE_FILE run --rm test-runner || true

    # Collect service logs
    log "Collecting service logs..."
    docker-compose -f $COMPOSE_FILE logs data-collector > $TEST_RESULTS_DIR/data-collector.log 2>&1
    docker-compose -f $COMPOSE_FILE logs api-gateway > $TEST_RESULTS_DIR/api-gateway.log 2>&1
    docker-compose -f $COMPOSE_FILE logs calculation-engine > $TEST_RESULTS_DIR/calculation-engine.log 2>&1

    success "Integration tests completed"
}

run_load_tests() {
    log "Running load tests..."

    # Ensure API gateway is running
    if ! wait_for_service api-gateway 8080 30; then
        error "API Gateway not available for load testing"
        return 1
    fi

    # Run load tests using Artillery or similar tool
    if command -v artillery &> /dev/null; then
        artillery run ./tests/load/api-load-test.yml --output $TEST_RESULTS_DIR/load-test-results.json
    else
        warning "Artillery not installed, skipping load tests"
    fi

    success "Load tests completed"
}

generate_test_report() {
    log "Generating test report..."

    cat > $TEST_RESULTS_DIR/test-report.html << EOF
<!DOCTYPE html>
<html>
<head>
    <title>Bloomberg Terminal Test Report</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        .header { background: #f4f4f4; padding: 20px; border-radius: 5px; }
        .section { margin: 20px 0; padding: 15px; border: 1px solid #ddd; }
        .success { color: green; }
        .error { color: red; }
        .warning { color: orange; }
    </style>
</head>
<body>
    <div class="header">
        <h1>TradeCaptain - Test Report</h1>
        <p>Generated on: $(date)</p>
    </div>

    <div class="section">
        <h2>Test Summary</h2>
        <p>This report contains results from unit tests, integration tests, and load tests.</p>
    </div>

    <div class="section">
        <h2>Test Files</h2>
        <ul>
EOF

    # List all test result files
    for file in $TEST_RESULTS_DIR/*; do
        if [ -f "$file" ]; then
            echo "            <li><a href=\"$(basename "$file")\">$(basename "$file")</a></li>" >> $TEST_RESULTS_DIR/test-report.html
        fi
    done

    cat >> $TEST_RESULTS_DIR/test-report.html << EOF
        </ul>
    </div>
</body>
</html>
EOF

    success "Test report generated at $TEST_RESULTS_DIR/test-report.html"
}

# Main execution
main() {
    log "Starting TradeCaptain test deployment..."

    # Parse command line arguments
    case "${1:-all}" in
        "cleanup")
            cleanup
            ;;
        "setup")
            setup_test_environment
            create_test_compose_file
            ;;
        "unit")
            setup_test_environment
            run_unit_tests
            ;;
        "integration")
            setup_test_environment
            create_test_compose_file
            run_integration_tests
            ;;
        "load")
            run_load_tests
            ;;
        "all")
            # Cleanup any previous runs
            cleanup

            # Setup environment
            setup_test_environment
            create_test_compose_file

            # Run all tests
            run_unit_tests
            run_integration_tests
            run_load_tests

            # Generate report
            generate_test_report

            # Cleanup
            cleanup
            ;;
        *)
            echo "Usage: $0 {setup|unit|integration|load|all|cleanup}"
            echo ""
            echo "Commands:"
            echo "  setup       - Set up test environment only"
            echo "  unit        - Run unit tests only"
            echo "  integration - Run integration tests only"
            echo "  load        - Run load tests only"
            echo "  all         - Run complete test suite (default)"
            echo "  cleanup     - Clean up test environment"
            exit 1
            ;;
    esac

    success "Test deployment completed successfully!"
}

# Trap cleanup on script exit
trap cleanup EXIT

# Execute main function
main "$@"