package observability

import (
	"runtime"
	"sort"
	"strconv"
	"sync"
	"time"
)

type HTTPMetric struct {
	Method          string  `json:"method"`
	Route           string  `json:"route"`
	Status          int     `json:"status"`
	Requests        uint64  `json:"requests"`
	TotalDurationMS float64 `json:"total_duration_ms"`
}

type DatabaseMetric struct {
	Operation       string  `json:"operation"`
	Calls           uint64  `json:"calls"`
	Errors          uint64  `json:"errors"`
	Rows            uint64  `json:"rows"`
	TotalDurationMS float64 `json:"total_duration_ms"`
}

type RuntimeMetric struct {
	Goroutines     int    `json:"goroutines"`
	HeapAllocBytes uint64 `json:"heap_alloc_bytes"`
	HeapInUseBytes uint64 `json:"heap_in_use_bytes"`
	SystemBytes    uint64 `json:"system_bytes"`
	CompletedGCs   uint32 `json:"completed_gcs"`
}

type Snapshot struct {
	HTTP     []HTTPMetric     `json:"http"`
	Database []DatabaseMetric `json:"database"`
	Runtime  RuntimeMetric    `json:"runtime"`
}

type Registry struct {
	mu       sync.RWMutex
	http     map[string]HTTPMetric
	database map[string]DatabaseMetric
}

func NewRegistry() *Registry {
	return &Registry{
		http:     make(map[string]HTTPMetric),
		database: make(map[string]DatabaseMetric),
	}
}

func (r *Registry) RecordHTTP(method, route string, status int, duration time.Duration) {
	key := method + "\x00" + route + "\x00" + strconv.Itoa(status)
	r.mu.Lock()
	metric := r.http[key]
	metric.Method, metric.Route, metric.Status = method, route, status
	metric.Requests++
	metric.TotalDurationMS += float64(duration) / float64(time.Millisecond)
	r.http[key] = metric
	r.mu.Unlock()
}

func (r *Registry) RecordDatabase(operation string, duration time.Duration, rows int, err error) {
	r.mu.Lock()
	metric := r.database[operation]
	metric.Operation = operation
	metric.Calls++
	metric.TotalDurationMS += float64(duration) / float64(time.Millisecond)
	if rows > 0 {
		metric.Rows += uint64(rows)
	}
	if err != nil {
		metric.Errors++
	}
	r.database[operation] = metric
	r.mu.Unlock()
}

func (r *Registry) Snapshot() Snapshot {
	r.mu.RLock()
	httpMetrics := make([]HTTPMetric, 0, len(r.http))
	for _, metric := range r.http {
		httpMetrics = append(httpMetrics, metric)
	}
	databaseMetrics := make([]DatabaseMetric, 0, len(r.database))
	for _, metric := range r.database {
		databaseMetrics = append(databaseMetrics, metric)
	}
	r.mu.RUnlock()
	sort.Slice(httpMetrics, func(i, j int) bool {
		if httpMetrics[i].Route != httpMetrics[j].Route {
			return httpMetrics[i].Route < httpMetrics[j].Route
		}
		if httpMetrics[i].Method != httpMetrics[j].Method {
			return httpMetrics[i].Method < httpMetrics[j].Method
		}
		return httpMetrics[i].Status < httpMetrics[j].Status
	})
	sort.Slice(databaseMetrics, func(i, j int) bool { return databaseMetrics[i].Operation < databaseMetrics[j].Operation })
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	return Snapshot{
		HTTP:     httpMetrics,
		Database: databaseMetrics,
		Runtime: RuntimeMetric{
			Goroutines:     runtime.NumGoroutine(),
			HeapAllocBytes: memory.HeapAlloc,
			HeapInUseBytes: memory.HeapInuse,
			SystemBytes:    memory.Sys,
			CompletedGCs:   memory.NumGC,
		},
	}
}
