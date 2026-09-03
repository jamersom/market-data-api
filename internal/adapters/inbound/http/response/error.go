package response

type ErrorDetail struct {
	Code           string `json:"code"`
	Message        string `json:"message"`
	Field          string `json:"field,omitempty"`
	Value          string `json:"value,omitempty"`
	ExpectedFormat string `json:"expectedFormat,omitempty"`
	Retryable      bool   `json:"retryable"`
}

type Error struct {
	Error ErrorDetail `json:"error"`
}
