package response

type ComparisonEnvelope struct {
	Data Comparison         `json:"data"`
	Meta ComparisonMetadata `json:"meta"`
}

type Comparison struct {
	Assets       []AssetComparison  `json:"assets"`
	Ranking      ComparisonRanking  `json:"ranking"`
	Correlations []AssetCorrelation `json:"correlations,omitempty"`
	Benchmark    *AssetComparison   `json:"benchmark,omitempty"`
}

type AssetComparison struct {
	Ticker               string            `json:"ticker"`
	Status               string            `json:"status"`
	AvailableFrom        *string           `json:"availableFrom"`
	AvailableTo          *string           `json:"availableTo"`
	InitialPrice         *string           `json:"initialPrice,omitempty"`
	FinalPrice           *string           `json:"finalPrice,omitempty"`
	AbsoluteReturn       *string           `json:"absoluteReturn,omitempty"`
	PercentageReturn     *float64          `json:"percentageReturn,omitempty"`
	AnnualizedVolatility *float64          `json:"annualizedVolatility,omitempty"`
	MaximumDrawdown      *float64          `json:"maximumDrawdown,omitempty"`
	AverageDailyVolume   *string           `json:"averageDailyVolume,omitempty"`
	LowestPrice          *string           `json:"lowestPrice,omitempty"`
	HighestPrice         *string           `json:"highestPrice,omitempty"`
	BestDay              *DailyPerformance `json:"bestDay,omitempty"`
	WorstDay             *DailyPerformance `json:"worstDay,omitempty"`
	Series               []ComparisonPoint `json:"series,omitempty"`
	Message              string            `json:"message,omitempty"`
}

type DailyPerformance struct {
	Date   string  `json:"date"`
	Return float64 `json:"return"`
}

type ComparisonPoint struct {
	Date                  string  `json:"date"`
	ClosePrice            string  `json:"closePrice"`
	NormalizedPerformance float64 `json:"normalizedPerformance"`
}

type ComparisonRanking struct {
	HighestReturn    string `json:"highestReturn,omitempty"`
	LowestVolatility string `json:"lowestVolatility,omitempty"`
	LowestDrawdown   string `json:"lowestDrawdown,omitempty"`
}

type AssetCorrelation struct {
	TickerA string  `json:"tickerA"`
	TickerB string  `json:"tickerB"`
	Value   float64 `json:"value"`
}

type ComparisonMetadata struct {
	RequestedTickers   []string `json:"requestedTickers"`
	From               string   `json:"from"`
	To                 string   `json:"to"`
	MarketType         int      `json:"marketType"`
	Metrics            []string `json:"metrics"`
	PriceAdjustment    string   `json:"priceAdjustment"`
	Currency           string   `json:"currency"`
	Source             string   `json:"source"`
	CalculationVersion string   `json:"calculationVersion"`
}
