package response

type IntelligenceEnvelope struct {
	Data Intelligence         `json:"data"`
	Meta IntelligenceMetadata `json:"meta"`
}

type Intelligence struct {
	Ticker    string                 `json:"ticker"`
	Price     IntelligencePrice      `json:"price"`
	Returns   IntelligenceReturns    `json:"returns"`
	Trend     IntelligenceTrend      `json:"trend"`
	Momentum  IntelligenceMomentum   `json:"momentum"`
	Risk      IntelligenceRisk       `json:"risk"`
	Liquidity IntelligenceLiquidity  `json:"liquidity"`
	Benchmark *IntelligenceBenchmark `json:"benchmark,omitempty"`
	Signals   []IntelligenceSignal   `json:"signals"`
}
type IntelligencePrice struct {
	Close    *string `json:"close"`
	Currency string  `json:"currency"`
}
type IntelligenceReturns struct {
	Return7D *float64 `json:"return_7d"`
}
type IntelligenceTrend struct {
	SMA20         *string  `json:"sma20"`
	SMA50         *string  `json:"sma50"`
	DistanceSMA20 *float64 `json:"distance_sma20"`
}
type IntelligenceMomentum struct {
	RSI14         *float64                   `json:"rsi14"`
	RSIPercentile *IntelligenceRSIPercentile `json:"rsi14_percentile"`
}
type IntelligenceRSIPercentile struct {
	Value        float64                        `json:"value"`
	Window       string                         `json:"window"`
	Observations int                            `json:"observations"`
	Coverage     IntelligencePercentileCoverage `json:"coverage"`
}
type IntelligencePercentileCoverage struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Complete bool   `json:"complete"`
}
type IntelligenceRisk struct {
	Volatility30D       *float64 `json:"volatility_30d"`
	DrawdownCurrent     *float64 `json:"drawdown_current"`
	MaximumDrawdown252D *float64 `json:"maximum_drawdown_252d"`
}
type IntelligenceLiquidity struct {
	AverageDailyVolume20D *string `json:"average_daily_volume_20d"`
}
type IntelligenceBenchmark struct {
	Ticker           string                                `json:"ticker"`
	Returns          IntelligenceBenchmarkReturns          `json:"returns"`
	RelativeStrength IntelligenceBenchmarkRelativeStrength `json:"relative_strength"`
}
type IntelligenceBenchmarkReturns struct {
	Return7D *float64 `json:"return_7d"`
}
type IntelligenceBenchmarkRelativeStrength struct {
	Return7DPP *float64 `json:"return_7d_pp"`
}
type IntelligenceSignal struct {
	ID       string                      `json:"id"`
	Status   string                      `json:"status"`
	Severity string                      `json:"severity"`
	Evidence *IntelligenceSignalEvidence `json:"evidence,omitempty"`
}
type IntelligenceSignalEvidence struct {
	RSI14     *float64 `json:"rsi14,omitempty"`
	Threshold *float64 `json:"threshold,omitempty"`
	Price     *float64 `json:"price,omitempty"`
	SMA20     *float64 `json:"sma20,omitempty"`
	SMA50     *float64 `json:"sma50,omitempty"`
}
type IntelligenceUnavailable struct {
	Field  string `json:"field"`
	Reason string `json:"reason"`
}
type IntelligenceMetadata struct {
	Calendar           *IntelligenceCalendar     `json:"calendar,omitempty"`
	Status             string                    `json:"status"`
	MarketType         int                       `json:"market_type"`
	RequestedAsOf      *string                   `json:"requested_as_of,omitempty"`
	AsOf               string                    `json:"as_of"`
	Source             string                    `json:"source"`
	PriceAdjustment    string                    `json:"price_adjustment"`
	WindowUnit         string                    `json:"window_unit"`
	PercentageUnit     string                    `json:"percentage_unit"`
	CalculationVersion string                    `json:"calculation_version,omitempty"`
	RulesetVersion     string                    `json:"ruleset_version,omitempty"`
	DataVersion        string                    `json:"data_version,omitempty"`
	RSISeedFrom        string                    `json:"rsi_seed_from,omitempty"`
	Unavailable        []IntelligenceUnavailable `json:"unavailable"`
}

type IntelligenceCalendar struct {
	Source           string                         `json:"source"`
	Version          string                         `json:"version"`
	Policy           string                         `json:"policy"`
	OfficialVerified bool                           `json:"official_verified"`
	Coverage         []IntelligenceCalendarCoverage `json:"coverage,omitempty"`
}
type IntelligenceCalendarCoverage struct {
	Year               int    `json:"year"`
	From               string `json:"observed_from"`
	To                 string `json:"observed_to"`
	IntegrityValidated bool   `json:"import_integrity_validated"`
	Version            string `json:"version"`
}
