package response

type Quote struct {
	Ticker                string   `json:"ticker"`
	TradingDate           string   `json:"tradingDate"`
	BDICode               string   `json:"bdiCode"`
	MarketType            int      `json:"marketType"`
	MarketTypeDescription string   `json:"marketTypeDescription"`
	ShortName             string   `json:"shortName"`
	Specification         string   `json:"specification"`
	Term                  string   `json:"term,omitempty"`
	Currency              string   `json:"currency"`
	OpenPrice             string   `json:"openPrice"`
	HighPrice             string   `json:"highPrice"`
	LowPrice              string   `json:"lowPrice"`
	AveragePrice          string   `json:"averagePrice"`
	ClosePrice            string   `json:"closePrice"`
	PreviousClose         *string  `json:"previousClose,omitempty"`
	AbsoluteChange        *string  `json:"absoluteChange,omitempty"`
	PercentageChange      *float64 `json:"percentageChange,omitempty"`
	IntradayRange         string   `json:"intradayRange"`
	BestBidPrice          string   `json:"bestBidPrice"`
	BestAskPrice          string   `json:"bestAskPrice"`
	BidAskSpread          string   `json:"bidAskSpread"`
	TradeCount            int      `json:"tradeCount"`
	TradedQuantity        int64    `json:"tradedQuantity"`
	TradedVolume          string   `json:"tradedVolume"`
	QuoteFactor           int      `json:"quoteFactor"`
	ISIN                  string   `json:"isin"`
	DistributionNumber    int      `json:"distributionNumber"`
}

type Metadata struct {
	Ticker          string `json:"ticker"`
	MarketType      int    `json:"marketType"`
	From            string `json:"from,omitempty"`
	To              string `json:"to,omitempty"`
	Records         int    `json:"records"`
	TotalRecords    int64  `json:"totalRecords"`
	Limit           int    `json:"limit,omitempty"`
	Offset          int    `json:"offset,omitempty"`
	HasMore         bool   `json:"hasMore"`
	Order           string `json:"order,omitempty"`
	Source          string `json:"source"`
	SourceURL       string `json:"sourceUrl,omitempty"`
	ParserVersion   string `json:"parserVersion,omitempty"`
	LayoutVersion   string `json:"layoutVersion,omitempty"`
	PublishedAt     string `json:"publishedAt,omitempty"`
	PriceAdjustment string `json:"priceAdjustment"`
}

type QuoteEnvelope struct {
	Data Quote    `json:"data"`
	Meta Metadata `json:"meta"`
}
type QuoteHistoryEnvelope struct {
	Data []Quote  `json:"data"`
	Meta Metadata `json:"meta"`
}
