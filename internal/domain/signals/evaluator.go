package signals

type Status string

const (
	StatusTriggered    Status = "triggered"
	StatusNotTriggered Status = "not_triggered"
	StatusUnavailable  Status = "unavailable"
)

type Severity string

const (
	SeverityInfo    Severity = "info"
	SeverityWarning Severity = "warning"
)

type Indicators struct {
	PriceCents *float64
	RSI14      *float64
	SMA20Cents *float64
	SMA50Cents *float64
}

type Evidence struct {
	RSI14      *float64
	Threshold  *float64
	PriceCents *float64
	SMA20Cents *float64
	SMA50Cents *float64
}

type Evaluation struct {
	ID       string
	Status   Status
	Severity Severity
	Evidence *Evidence
}

type Rule struct {
	ID       string
	Severity Severity
	evaluate func(Indicators) (bool, Evidence, bool)
}

type Ruleset struct {
	Version string
	Rules   []Rule
}

type Evaluator struct {
	ruleset Ruleset
}

func NewEvaluator(ruleset Ruleset) Evaluator {
	return Evaluator{ruleset: ruleset}
}

func (e Evaluator) Version() string {
	return e.ruleset.Version
}

func (e Evaluator) Evaluate(indicators Indicators) []Evaluation {
	evaluations := make([]Evaluation, 0, len(e.ruleset.Rules))
	for _, rule := range e.ruleset.Rules {
		triggered, evidence, available := rule.evaluate(indicators)
		status := StatusNotTriggered
		var resultEvidence *Evidence
		if !available {
			status = StatusUnavailable
		} else {
			if triggered {
				status = StatusTriggered
			}
			resultEvidence = &evidence
		}
		evaluations = append(evaluations, Evaluation{
			ID:       rule.ID,
			Status:   status,
			Severity: rule.Severity,
			Evidence: resultEvidence,
		})
	}
	return evaluations
}
