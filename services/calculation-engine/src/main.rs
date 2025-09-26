use anyhow::Result;
use tracing::{info, error};
use tracing_subscriber;

mod lib;
mod financial;
mod risk;
mod technical;
mod portfolio;

use crate::financial::FinancialCalculator;
use crate::risk::RiskCalculator;
use crate::technical::TechnicalIndicators;
use crate::portfolio::PortfolioAnalyzer;

#[tokio::main]
async fn main() -> Result<()> {
    // Initialize tracing
    tracing_subscriber::init();

    info!("Starting Bloomberg Terminal Calculation Engine");

    // Initialize calculators
    let financial_calc = FinancialCalculator::new();
    let risk_calc = RiskCalculator::new();
    let technical_calc = TechnicalIndicators::new();
    let portfolio_analyzer = PortfolioAnalyzer::new();

    // Example calculations
    run_examples(&financial_calc, &risk_calc, &technical_calc, &portfolio_analyzer).await?;

    info!("Calculation engine running. Press Ctrl+C to stop.");

    // Keep the service running
    tokio::signal::ctrl_c().await?;

    info!("Shutting down calculation engine");
    Ok(())
}

async fn run_examples(
    financial: &FinancialCalculator,
    risk: &RiskCalculator,
    technical: &TechnicalIndicators,
    portfolio: &PortfolioAnalyzer,
) -> Result<()> {
    // Black-Scholes option pricing example
    let option_price = financial.black_scholes(100.0, 105.0, 0.25, 0.05, 0.2)?;
    info!("Black-Scholes option price: ${:.2}", option_price);

    // VaR calculation example
    let returns = vec![0.01, -0.02, 0.015, -0.01, 0.005, -0.008, 0.012];
    let var_95 = risk.value_at_risk(&returns, 0.95)?;
    info!("95% VaR: {:.4}", var_95);

    // Moving average example
    let prices = vec![100.0, 102.0, 101.0, 103.0, 105.0, 104.0, 106.0];
    let sma = technical.simple_moving_average(&prices, 5)?;
    info!("Simple Moving Average: {:.2}", sma);

    // Portfolio Sharpe ratio example
    let portfolio_returns = vec![0.08, 0.12, -0.05, 0.15, 0.10];
    let sharpe = portfolio.sharpe_ratio(&portfolio_returns, 0.03)?;
    info!("Portfolio Sharpe Ratio: {:.4}", sharpe);

    Ok(())
}