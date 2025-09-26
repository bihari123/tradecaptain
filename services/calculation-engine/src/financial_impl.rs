use anyhow::Result;
use statrs::distribution::{ContinuousCDF, Normal};

impl FinancialCalculator {
    pub fn black_scholes(
        &self,
        spot: f64,
        strike: f64,
        time_to_expiry: f64,
        risk_free_rate: f64,
        volatility: f64,
    ) -> Result<f64> {
        if spot <= 0.0 || strike <= 0.0 || time_to_expiry <= 0.0 || volatility <= 0.0 {
            return Err(anyhow::anyhow!("Invalid parameters for Black-Scholes"));
        }

        let d1 = (spot.ln() - strike.ln() + (risk_free_rate + 0.5 * volatility.powi(2)) * time_to_expiry)
            / (volatility * time_to_expiry.sqrt());

        let d2 = d1 - volatility * time_to_expiry.sqrt();

        let call_price = spot * self.normal_dist.cdf(d1)
            - strike * (-risk_free_rate * time_to_expiry).exp() * self.normal_dist.cdf(d2);

        Ok(call_price)
    }

    pub fn black_scholes_put(
        &self,
        spot: f64,
        strike: f64,
        time_to_expiry: f64,
        risk_free_rate: f64,
        volatility: f64,
    ) -> Result<f64> {
        if spot <= 0.0 || strike <= 0.0 || time_to_expiry <= 0.0 || volatility <= 0.0 {
            return Err(anyhow::anyhow!("Invalid parameters for Black-Scholes put"));
        }

        let d1 = (spot.ln() - strike.ln() + (risk_free_rate + 0.5 * volatility.powi(2)) * time_to_expiry)
            / (volatility * time_to_expiry.sqrt());

        let d2 = d1 - volatility * time_to_expiry.sqrt();

        let put_price = strike * (-risk_free_rate * time_to_expiry).exp() * self.normal_dist.cdf(-d2)
            - spot * self.normal_dist.cdf(-d1);

        Ok(put_price)
    }

    pub fn delta_call(&self, spot: f64, strike: f64, time_to_expiry: f64, risk_free_rate: f64, volatility: f64) -> Result<f64> {
        let d1 = (spot.ln() - strike.ln() + (risk_free_rate + 0.5 * volatility.powi(2)) * time_to_expiry)
            / (volatility * time_to_expiry.sqrt());

        Ok(self.normal_dist.cdf(d1))
    }

    pub fn bond_price(&self, face_value: f64, coupon_rate: f64, yield_rate: f64, periods: i32) -> Result<f64> {
        if face_value <= 0.0 || periods <= 0 {
            return Err(anyhow::anyhow!("Invalid bond parameters"));
        }

        let coupon_payment = face_value * coupon_rate;
        let mut present_value = 0.0;

        // Present value of coupon payments
        for i in 1..=periods {
            present_value += coupon_payment / (1.0 + yield_rate).powi(i);
        }

        // Present value of face value
        present_value += face_value / (1.0 + yield_rate).powi(periods);

        Ok(present_value)
    }

    pub fn bond_duration(&self, face_value: f64, coupon_rate: f64, yield_rate: f64, periods: i32) -> Result<f64> {
        let bond_price = self.bond_price(face_value, coupon_rate, yield_rate, periods)?;
        let coupon_payment = face_value * coupon_rate;

        let mut weighted_time = 0.0;

        // Weighted time for coupon payments
        for i in 1..=periods {
            let pv = coupon_payment / (1.0 + yield_rate).powi(i);
            weighted_time += (i as f64) * pv;
        }

        // Weighted time for face value
        let pv_face = face_value / (1.0 + yield_rate).powi(periods);
        weighted_time += (periods as f64) * pv_face;

        Ok(weighted_time / bond_price)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use approx::assert_relative_eq;

    #[test]
    fn test_black_scholes_call() {
        let calc = FinancialCalculator::new();

        // Standard Black-Scholes test case
        let result = calc.black_scholes(100.0, 100.0, 1.0, 0.05, 0.20).unwrap();

        // Expected value approximately 10.45
        assert_relative_eq!(result, 10.45, epsilon = 0.01);
    }

    #[test]
    fn test_black_scholes_put() {
        let calc = FinancialCalculator::new();

        let result = calc.black_scholes_put(100.0, 100.0, 1.0, 0.05, 0.20).unwrap();

        // Expected value approximately 5.57 (using put-call parity)
        assert_relative_eq!(result, 5.57, epsilon = 0.01);
    }

    #[test]
    fn test_bond_pricing() {
        let calc = FinancialCalculator::new();

        // 5% coupon, 4% yield, 5 years
        let result = calc.bond_price(1000.0, 0.05, 0.04, 5).unwrap();

        // Should be trading at premium (above par)
        assert!(result > 1000.0);
        assert_relative_eq!(result, 1044.52, epsilon = 0.01);
    }

    #[test]
    fn test_bond_duration() {
        let calc = FinancialCalculator::new();

        let result = calc.bond_duration(1000.0, 0.05, 0.04, 5).unwrap();

        // Duration should be less than maturity for coupon bonds
        assert!(result < 5.0);
        assert_relative_eq!(result, 4.47, epsilon = 0.01);
    }

    #[test]
    fn test_delta_calculation() {
        let calc = FinancialCalculator::new();

        let result = calc.delta_call(100.0, 100.0, 1.0, 0.05, 0.20).unwrap();

        // At-the-money call delta should be around 0.63
        assert_relative_eq!(result, 0.63, epsilon = 0.01);
    }
}