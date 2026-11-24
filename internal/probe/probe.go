package probe

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strconv"
	"sync"
	"time"
)

// Result describes one network connection attempt.
type Result struct {
	Target    string  `json:"target"`
	Success   bool    `json:"success"`
	LatencyMS float64 `json:"latencyMs"`
	Error     string  `json:"error,omitempty"`
}

// Sample groups all probe attempts made during one measurement tick.
type Sample struct {
	Time      time.Time `json:"time"`
	Results   []Result  `json:"results"`
	LatencyMS float64   `json:"latencyMs"`
	LossPct   float64   `json:"lossPct"`
}

// Engine performs small TCP handshakes against several independent endpoints.
// This works without administrator privileges on Linux and Windows.
type Engine struct {
	Targets []string
	Port    int
	Timeout time.Duration
}

// Measure probes all targets concurrently.
func (e Engine) Measure(ctx context.Context) Sample {
	sample := Sample{Time: time.Now(), Results: make([]Result, len(e.Targets))}
	var wg sync.WaitGroup

	for i, target := range e.Targets {
		wg.Add(1)
		go func(index int, host string) {
			defer wg.Done()
			sample.Results[index] = e.measureOne(ctx, host)
		}(i, target)
	}

	wg.Wait()
	sample.LatencyMS, sample.LossPct = summarize(sample.Results)
	return sample
}

func (e Engine) measureOne(ctx context.Context, target string) Result {
	result := Result{Target: target}
	address := net.JoinHostPort(target, strconv.Itoa(e.Port))
	dialer := net.Dialer{Timeout: e.Timeout}
	started := time.Now()
	conn, err := dialer.DialContext(ctx, "tcp", address)
	result.LatencyMS = float64(time.Since(started).Microseconds()) / 1000
	if err != nil {
		result.Error = shortError(err)
		return result
	}
	_ = conn.Close()
	result.Success = true
	return result
}

func summarize(results []Result) (latencyMS, lossPct float64) {
	if len(results) == 0 {
		return 0, 100
	}

	latencies := make([]float64, 0, len(results))
	for _, result := range results {
		if result.Success {
			latencies = append(latencies, result.LatencyMS)
		}
	}
	lossPct = float64(len(results)-len(latencies)) / float64(len(results)) * 100
	if len(latencies) == 0 {
		return 0, lossPct
	}
	sort.Float64s(latencies)
	middle := len(latencies) / 2
	if len(latencies)%2 == 0 {
		latencyMS = (latencies[middle-1] + latencies[middle]) / 2
	} else {
		latencyMS = latencies[middle]
	}
	return latencyMS, lossPct
}

func shortError(err error) string {
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		return "timeout"
	}
	return fmt.Sprintf("%v", err)
}
