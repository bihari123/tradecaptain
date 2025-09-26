pub mod financial;
pub mod risk;
pub mod technical;
pub mod portfolio;
pub mod cache;
pub mod persistence;

pub use financial::FinancialCalculator;
pub use risk::RiskCalculator;
pub use technical::TechnicalIndicators;
pub use portfolio::PortfolioAnalyzer;
pub use cache::{L1Cache, UltraFastChannel, LockFreeQueue, MarketDataCached};
pub use persistence::{UltraFastDB, PortfolioState, TimeSeriesPoint};

use std::ffi::{CStr, CString};
use std::os::raw::{c_char, c_double, c_int};

// C FFI exports for Go integration
#[no_mangle]
pub extern "C" fn black_scholes_c(
    spot: c_double,
    strike: c_double,
    time_to_expiry: c_double,
    risk_free_rate: c_double,
    volatility: c_double,
) -> c_double {
    let calc = FinancialCalculator::new();
    calc.black_scholes(spot, strike, time_to_expiry, risk_free_rate, volatility)
        .unwrap_or(0.0)
}

#[no_mangle]
pub extern "C" fn calculate_var_c(
    returns_ptr: *const c_double,
    length: c_int,
    confidence: c_double,
) -> c_double {
    if returns_ptr.is_null() || length <= 0 {
        return 0.0;
    }

    let returns = unsafe {
        std::slice::from_raw_parts(returns_ptr, length as usize)
    };

    let calc = RiskCalculator::new();
    calc.value_at_risk(&returns.to_vec(), confidence)
        .unwrap_or(0.0)
}

#[no_mangle]
pub extern "C" fn simple_moving_average_c(
    prices_ptr: *const c_double,
    length: c_int,
    period: c_int,
) -> c_double {
    if prices_ptr.is_null() || length <= 0 || period <= 0 {
        return 0.0;
    }

    let prices = unsafe {
        std::slice::from_raw_parts(prices_ptr, length as usize)
    };

    let calc = TechnicalIndicators::new();
    calc.simple_moving_average(&prices.to_vec(), period as usize)
        .unwrap_or(0.0)
}