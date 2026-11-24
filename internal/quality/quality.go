package quality

import (
	"math"
	"sort"
	"time"

	"github.com/KuRLiC/constatra/internal/probe"
)

// Level is the user-facing quality category. Its ordering is intentional.
type Level string

const (
	Checking  Level = "checking"
	Excellent Level = "excellent"
	Good      Level = "good"
	Poor      Level = "poor"
	Critical  Level = "critical"
	Offline   Level = "offline"
)

// Stats is the quality summary for a rolling time window.
type Stats struct {
	Level           Level   `json:"level"`
	Label           string  `json:"label"`
	Score           int     `json:"score"`
	MedianLatencyMS float64 `json:"medianLatencyMs"`
	P90LatencyMS    float64 `json:"p90LatencyMs"`
	JitterMS        float64 `json:"jitterMs"`
	LossPct         float64 `json:"lossPct"`
	AvailabilityPct float64 `json:"availabilityPct"`
	Successful      int     `json:"successful"`
	Total           int     `json:"total"`
	WindowSeconds   int     `json:"windowSeconds"`
	ObservedSeconds int     `json:"observedSeconds"`
}

// Calculate summarizes samples newer than now-window. When fastOffline is true,
// two consecutive completely failed ticks immediately produce OFFLINE.
func Calculate(samples []probe.Sample, now time.Time, window time.Duration, fastOffline bool) Stats {
	stats := Stats{Level: Checking, Label: "CHECKING", WindowSeconds: int(window.Seconds())}
	cutoff := now.Add(-window)
	windowSamples := make([]probe.Sample, 0, len(samples))
	latencies := make([]float64, 0, len(samples)*3)
	availableTicks := 0

	for _, sample := range samples {
		if sample.Time.Before(cutoff) {
			continue
		}
		windowSamples = append(windowSamples, sample)
	}
	sort.Slice(windowSamples, func(i, j int) bool {
		return windowSamples[i].Time.Before(windowSamples[j].Time)
	})

	for _, sample := range windowSamples {
		tickAvailable := false
		for _, result := range sample.Results {
			stats.Total++
			if result.Success {
				stats.Successful++
				tickAvailable = true
				latencies = append(latencies, result.LatencyMS)
			}
		}
		if tickAvailable {
			availableTicks++
		}
	}

	if stats.Total == 0 || len(windowSamples) == 0 {
		return stats
	}
	stats.ObservedSeconds = int(math.Ceil(now.Sub(windowSamples[0].Time).Seconds())) + 1

	stats.LossPct = float64(stats.Total-stats.Successful) / float64(stats.Total) * 100
	stats.AvailabilityPct = float64(availableTicks) / float64(len(windowSamples)) * 100
	if stats.Successful == 0 || (fastOffline && offlineFor(windowSamples, now) >= 2*time.Second) {
		stats.Level = Offline
		stats.Label = "OFFLINE"
		return stats
	}

	sort.Float64s(latencies)
	stats.MedianLatencyMS = percentile(latencies, 0.50)
	stats.P90LatencyMS = percentile(latencies, 0.90)
	stats.JitterMS = temporalJitter(windowSamples)

	score := 100.0
	score -= math.Min(stats.LossPct*1.5, 65)
	score -= latencyPenalty(stats.P90LatencyMS)
	score -= jitterPenalty(stats.JitterMS)
	score -= math.Min((100-stats.AvailabilityPct)*0.4, 30)
	stats.Score = int(math.Round(math.Max(0, math.Min(100, score))))
	if !fastOffline && stats.ObservedSeconds < min(10, stats.WindowSeconds) {
		return stats
	}

	switch {
	case stats.Score < 35 || stats.LossPct >= 50 || stats.AvailabilityPct < 70 || stats.P90LatencyMS >= 700:
		stats.Level, stats.Label = Critical, "CRITICAL"
	case stats.Score < 60 || stats.LossPct >= 20 || stats.P90LatencyMS >= 350 || stats.JitterMS >= 150:
		stats.Level, stats.Label = Poor, "POOR"
	case stats.Score < 80 || stats.LossPct >= 5 || stats.P90LatencyMS >= 160 || stats.JitterMS >= 60:
		stats.Level, stats.Label = Good, "GOOD"
	default:
		stats.Level, stats.Label = Excellent, "EXCELLENT"
	}

	return stats
}

func offlineFor(samples []probe.Sample, now time.Time) time.Duration {
	for i := len(samples) - 1; i >= 0; i-- {
		for _, result := range samples[i].Results {
			if result.Success {
				return now.Sub(samples[i].Time)
			}
		}
	}
	return time.Duration(math.MaxInt64)
}

func percentile(sortedValues []float64, p float64) float64 {
	if len(sortedValues) == 0 {
		return 0
	}
	index := int(math.Ceil(p*float64(len(sortedValues)))) - 1
	if index < 0 {
		index = 0
	}
	if index >= len(sortedValues) {
		index = len(sortedValues) - 1
	}
	return sortedValues[index]
}

func temporalJitter(samples []probe.Sample) float64 {
	previous := make(map[string]float64)
	differences := make([]float64, 0, len(samples))
	for _, sample := range samples {
		for _, result := range sample.Results {
			if !result.Success {
				delete(previous, result.Target)
				continue
			}
			if value, ok := previous[result.Target]; ok {
				differences = append(differences, math.Abs(result.LatencyMS-value))
			}
			previous[result.Target] = result.LatencyMS
		}
	}
	if len(differences) == 0 {
		return 0
	}
	total := 0.0
	for _, difference := range differences {
		total += difference
	}
	return total / float64(len(differences))
}

func latencyPenalty(latencyMS float64) float64 {
	switch {
	case latencyMS <= 80:
		return latencyMS / 80 * 5
	case latencyMS <= 160:
		return 5 + (latencyMS-80)/80*12
	case latencyMS <= 350:
		return 17 + (latencyMS-160)/190*20
	case latencyMS <= 700:
		return 37 + (latencyMS-350)/350*18
	default:
		return 60
	}
}

func jitterPenalty(jitterMS float64) float64 {
	if jitterMS <= 20 {
		return jitterMS / 20 * 3
	}
	return math.Min(3+(jitterMS-20)/130*17, 25)
}
