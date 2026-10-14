package signals

const (
	RulesetV1Version       = "1.0"
	RSIOverboughtThreshold = 70.0
	RSIOversoldThreshold   = 30.0
)

func RulesetV1() Ruleset {
	return Ruleset{
		Version: RulesetV1Version,
		Rules: []Rule{
			{
				ID:       "rsi_overbought",
				Severity: SeverityWarning,
				evaluate: func(indicators Indicators) (bool, Evidence, bool) {
					if indicators.RSI14 == nil {
						return false, Evidence{}, false
					}
					threshold := RSIOverboughtThreshold
					return *indicators.RSI14 >= threshold, Evidence{RSI14: indicators.RSI14, Threshold: &threshold}, true
				},
			},
			{
				ID:       "rsi_oversold",
				Severity: SeverityWarning,
				evaluate: func(indicators Indicators) (bool, Evidence, bool) {
					if indicators.RSI14 == nil {
						return false, Evidence{}, false
					}
					threshold := RSIOversoldThreshold
					return *indicators.RSI14 <= threshold, Evidence{RSI14: indicators.RSI14, Threshold: &threshold}, true
				},
			},
			comparisonRule("price_above_sma20", SeverityInfo, func(indicators Indicators) (*float64, *float64) {
				return indicators.PriceCents, indicators.SMA20Cents
			}, func(left, right float64) bool { return left > right }, func(left, right *float64) Evidence {
				return Evidence{PriceCents: left, SMA20Cents: right}
			}),
			comparisonRule("price_below_sma20", SeverityInfo, func(indicators Indicators) (*float64, *float64) {
				return indicators.PriceCents, indicators.SMA20Cents
			}, func(left, right float64) bool { return left < right }, func(left, right *float64) Evidence {
				return Evidence{PriceCents: left, SMA20Cents: right}
			}),
			comparisonRule("sma20_above_sma50", SeverityInfo, func(indicators Indicators) (*float64, *float64) {
				return indicators.SMA20Cents, indicators.SMA50Cents
			}, func(left, right float64) bool { return left > right }, func(left, right *float64) Evidence {
				return Evidence{SMA20Cents: left, SMA50Cents: right}
			}),
			comparisonRule("sma20_below_sma50", SeverityInfo, func(indicators Indicators) (*float64, *float64) {
				return indicators.SMA20Cents, indicators.SMA50Cents
			}, func(left, right float64) bool { return left < right }, func(left, right *float64) Evidence {
				return Evidence{SMA20Cents: left, SMA50Cents: right}
			}),
		},
	}
}

func comparisonRule(
	id string,
	severity Severity,
	values func(Indicators) (*float64, *float64),
	condition func(float64, float64) bool,
	evidence func(*float64, *float64) Evidence,
) Rule {
	return Rule{
		ID:       id,
		Severity: severity,
		evaluate: func(indicators Indicators) (bool, Evidence, bool) {
			left, right := values(indicators)
			if left == nil || right == nil {
				return false, Evidence{}, false
			}
			return condition(*left, *right), evidence(left, right), true
		},
	}
}
