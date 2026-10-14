package signals

import "testing"

func number(value float64) *float64 { return &value }

func evaluationsByID(evaluations []Evaluation) map[string]Evaluation {
	result := make(map[string]Evaluation, len(evaluations))
	for _, evaluation := range evaluations {
		result[evaluation.ID] = evaluation
	}
	return result
}

func TestRulesetV1RSIBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name       string
		rsi        float64
		overbought Status
		oversold   Status
	}{
		{name: "above 70", rsi: 71, overbought: StatusTriggered, oversold: StatusNotTriggered},
		{name: "exactly 70", rsi: 70, overbought: StatusTriggered, oversold: StatusNotTriggered},
		{name: "neutral", rsi: 50, overbought: StatusNotTriggered, oversold: StatusNotTriggered},
		{name: "exactly 30", rsi: 30, overbought: StatusNotTriggered, oversold: StatusTriggered},
		{name: "below 30", rsi: 29, overbought: StatusNotTriggered, oversold: StatusTriggered},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := evaluationsByID(NewEvaluator(RulesetV1()).Evaluate(Indicators{RSI14: number(tc.rsi)}))
			if got["rsi_overbought"].Status != tc.overbought || got["rsi_oversold"].Status != tc.oversold {
				t.Fatalf("unexpected RSI evaluations: %+v", got)
			}
			if got["rsi_overbought"].Evidence == nil || *got["rsi_overbought"].Evidence.RSI14 != tc.rsi || *got["rsi_overbought"].Evidence.Threshold != RSIOverboughtThreshold {
				t.Fatalf("unexpected overbought evidence: %+v", got["rsi_overbought"].Evidence)
			}
			if got["rsi_oversold"].Evidence == nil || *got["rsi_oversold"].Evidence.Threshold != RSIOversoldThreshold {
				t.Fatalf("unexpected oversold evidence: %+v", got["rsi_oversold"].Evidence)
			}
		})
	}
}

func TestRulesetV1PriceAndMovingAverageEquality(t *testing.T) {
	for _, tc := range []struct {
		name                   string
		price, sma20, sma50    float64
		priceAbove, priceBelow Status
		sma20Above, sma20Below Status
	}{
		{name: "above", price: 110, sma20: 100, sma50: 90, priceAbove: StatusTriggered, priceBelow: StatusNotTriggered, sma20Above: StatusTriggered, sma20Below: StatusNotTriggered},
		{name: "below", price: 90, sma20: 100, sma50: 110, priceAbove: StatusNotTriggered, priceBelow: StatusTriggered, sma20Above: StatusNotTriggered, sma20Below: StatusTriggered},
		{name: "equal", price: 100, sma20: 100, sma50: 100, priceAbove: StatusNotTriggered, priceBelow: StatusNotTriggered, sma20Above: StatusNotTriggered, sma20Below: StatusNotTriggered},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := evaluationsByID(NewEvaluator(RulesetV1()).Evaluate(Indicators{
				PriceCents: number(tc.price), SMA20Cents: number(tc.sma20), SMA50Cents: number(tc.sma50),
			}))
			if got["price_above_sma20"].Status != tc.priceAbove || got["price_below_sma20"].Status != tc.priceBelow ||
				got["sma20_above_sma50"].Status != tc.sma20Above || got["sma20_below_sma50"].Status != tc.sma20Below {
				t.Fatalf("unexpected comparison evaluations: %+v", got)
			}
		})
	}
}

func TestRulesetV1UnavailableRulesRemainIndependent(t *testing.T) {
	got := evaluationsByID(NewEvaluator(RulesetV1()).Evaluate(Indicators{
		PriceCents: number(110), SMA20Cents: number(100),
	}))
	for _, id := range []string{"rsi_overbought", "rsi_oversold", "sma20_above_sma50", "sma20_below_sma50"} {
		if got[id].Status != StatusUnavailable || got[id].Evidence != nil {
			t.Fatalf("%s should be unavailable without evidence: %+v", id, got[id])
		}
	}
	if got["price_above_sma20"].Status != StatusTriggered || got["price_below_sma20"].Status != StatusNotTriggered {
		t.Fatalf("available price rules were not evaluated: %+v", got)
	}
}

func TestRulesetV1OrderAndVersionAreDeterministic(t *testing.T) {
	evaluator := NewEvaluator(RulesetV1())
	first := evaluator.Evaluate(Indicators{})
	second := evaluator.Evaluate(Indicators{})
	if evaluator.Version() != RulesetV1Version || len(first) != 6 || len(second) != 6 {
		t.Fatalf("version or rule count: %q, %d, %d", evaluator.Version(), len(first), len(second))
	}
	for i := range first {
		if first[i].ID != second[i].ID || first[i].Status != second[i].Status || first[i].Severity != second[i].Severity {
			t.Fatalf("non-deterministic evaluation at %d: %+v / %+v", i, first[i], second[i])
		}
	}
}
