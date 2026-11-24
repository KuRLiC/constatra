package quality

import (
	"fmt"
	"testing"
	"time"

	"github.com/KuRLiC/constatra/internal/probe"
)

func TestExcellentConnection(t *testing.T) {
	now := time.Now()
	samples := []probe.Sample{
		sample(now.Add(-2*time.Second), 42, 48, 55),
		sample(now.Add(-time.Second), 45, 50, 58),
		sample(now, 44, 52, 60),
	}
	stats := Calculate(samples, now, 3*time.Second, true)
	if stats.Level != Excellent {
		t.Fatalf("expected excellent, got %s (score %d)", stats.Level, stats.Score)
	}
	if stats.LossPct != 0 {
		t.Fatalf("expected no loss, got %.1f", stats.LossPct)
	}
}

func TestLossDegradesConnection(t *testing.T) {
	now := time.Now()
	samples := []probe.Sample{
		sample(now.Add(-2*time.Second), 45, -1, 48),
		sample(now.Add(-time.Second), 47, -1, 49),
		sample(now, 46, -1, 50),
	}
	stats := Calculate(samples, now, 3*time.Second, true)
	if stats.Level != Poor {
		t.Fatalf("expected poor for 33%% loss, got %s (score %d)", stats.Level, stats.Score)
	}
}

func TestTwoFailedTicksAreOffline(t *testing.T) {
	now := time.Now()
	samples := []probe.Sample{
		sample(now.Add(-2*time.Second), 45, 48, 50),
		sample(now.Add(-time.Second), -1, -1, -1),
		sample(now, -1, -1, -1),
	}
	stats := Calculate(samples, now, 3*time.Second, true)
	if stats.Level != Offline {
		t.Fatalf("expected offline, got %s", stats.Level)
	}
}

func TestStableDifferentTargetsDoNotCreateFalseJitter(t *testing.T) {
	now := time.Now()
	samples := []probe.Sample{
		sample(now.Add(-2*time.Second), 30, 80, 130),
		sample(now.Add(-time.Second), 30, 80, 130),
		sample(now, 30, 80, 130),
	}
	stats := Calculate(samples, now, 3*time.Second, true)
	if stats.JitterMS != 0 {
		t.Fatalf("expected temporal jitter 0, got %.1f", stats.JitterMS)
	}
}

func TestStabilityWarmsUp(t *testing.T) {
	now := time.Now()
	samples := []probe.Sample{sample(now, 40, 45, 50)}
	stats := Calculate(samples, now, 30*time.Second, false)
	if stats.Level != Checking {
		t.Fatalf("expected stability to be checking during warm-up, got %s", stats.Level)
	}
}

func sample(at time.Time, latencies ...float64) probe.Sample {
	results := make([]probe.Result, len(latencies))
	for i, latency := range latencies {
		results[i] = probe.Result{Target: fmt.Sprintf("target-%d", i), Success: latency >= 0, LatencyMS: max(latency, 0)}
	}
	return probe.Sample{Time: at, Results: results}
}
