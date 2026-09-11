package observability

import (
	"errors"
	"testing"
	"time"
)

func TestRegistryAggregatesBoundedDimensions(t *testing.T) {
	registry := NewRegistry()
	registry.RecordHTTP("GET", "GET /quotes/{ticker}", 200, 3*time.Millisecond)
	registry.RecordHTTP("GET", "GET /quotes/{ticker}", 200, 7*time.Millisecond)
	registry.RecordDatabase("quote_latest", 4*time.Millisecond, 1, nil)
	registry.RecordDatabase("quote_latest", 6*time.Millisecond, 0, errors.New("failed"))
	snapshot := registry.Snapshot()
	if len(snapshot.HTTP) != 1 || snapshot.HTTP[0].Requests != 2 || snapshot.HTTP[0].TotalDurationMS != 10 {
		t.Fatalf("unexpected HTTP metrics: %+v", snapshot.HTTP)
	}
	if len(snapshot.Database) != 1 || snapshot.Database[0].Calls != 2 || snapshot.Database[0].Errors != 1 || snapshot.Database[0].Rows != 1 || snapshot.Database[0].TotalDurationMS != 10 {
		t.Fatalf("unexpected database metrics: %+v", snapshot.Database)
	}
	if snapshot.Runtime.Goroutines < 1 || snapshot.Runtime.SystemBytes == 0 {
		t.Fatalf("runtime metrics unavailable: %+v", snapshot.Runtime)
	}
}
