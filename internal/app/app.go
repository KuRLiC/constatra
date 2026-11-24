package app

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/paul/constatra/internal/probe"
	"github.com/paul/constatra/internal/quality"
	"github.com/paul/constatra/internal/webui"
)

// Config contains validated runtime settings.
type Config struct {
	Interval        time.Duration
	ImmediateWindow time.Duration
	StabilityWindow time.Duration
	HistoryWindow   time.Duration
	Targets         []string
	Port            int
	Timeout         time.Duration
	Version         string
	Output          io.Writer
	Interactive     bool
}

// Snapshot is the complete state consumed by the browser dashboard.
type Snapshot struct {
	StartedAt  time.Time      `json:"startedAt"`
	Generated  time.Time      `json:"generatedAt"`
	Version    string         `json:"version"`
	IntervalMS int64          `json:"intervalMs"`
	Immediate  quality.Stats  `json:"immediate"`
	Stability  quality.Stats  `json:"stability"`
	Latest     *probe.Sample  `json:"latest,omitempty"`
	History    []probe.Sample `json:"history,omitempty"`
}

// Application owns measurements, rolling state, and HTTP handlers.
type Application struct {
	config        Config
	engine        probe.Engine
	startedAt     time.Time
	shutdownToken string
	requestStop   func()
	stopOnce      sync.Once
	done          chan struct{}

	stateMu sync.RWMutex
	history []probe.Sample
	latest  Snapshot

	subscribersMu sync.Mutex
	subscribers   map[chan []byte]struct{}
}

// New constructs an application and generates a per-run shutdown secret.
func New(config Config, requestStop func()) (*Application, error) {
	tokenBytes := make([]byte, 24)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("generate shutdown token: %w", err)
	}
	startedAt := time.Now()
	application := &Application{
		config: config,
		engine: probe.Engine{
			Targets: append([]string(nil), config.Targets...),
			Port:    config.Port,
			Timeout: config.Timeout,
		},
		startedAt:     startedAt,
		shutdownToken: hex.EncodeToString(tokenBytes),
		requestStop:   requestStop,
		done:          make(chan struct{}),
		subscribers:   make(map[chan []byte]struct{}),
	}
	application.latest = application.snapshot(startedAt, true)
	return application, nil
}

// Stop releases long-lived browser streams. It is safe to call more than once.
func (a *Application) Stop() {
	a.stopOnce.Do(func() { close(a.done) })
}

// Handler returns the local dashboard and API handler.
func (a *Application) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/state", a.handleState)
	mux.HandleFunc("GET /api/events", a.handleEvents)
	mux.HandleFunc("POST /api/shutdown", a.handleShutdown)
	mux.Handle("/", webui.Handler(a.shutdownToken))
	return securityHeaders(mux)
}

// RunMeasurements continuously measures, updates, logs, and publishes state.
func (a *Application) RunMeasurements(ctx context.Context) {
	ticker := time.NewTicker(a.config.Interval)
	defer ticker.Stop()

	for {
		a.measure(ctx)
		select {
		case <-ctx.Done():
			if a.config.Interactive {
				fmt.Fprintln(a.config.Output)
			}
			return
		case <-ticker.C:
		}
	}
}

func (a *Application) measure(ctx context.Context) {
	sample := a.engine.Measure(ctx)
	if ctx.Err() != nil {
		return
	}

	a.stateMu.Lock()
	a.history = append(a.history, sample)
	cutoff := sample.Time.Add(-a.config.HistoryWindow)
	first := 0
	for first < len(a.history) && a.history[first].Time.Before(cutoff) {
		first++
	}
	if first > 0 {
		trimmed := make([]probe.Sample, len(a.history)-first)
		copy(trimmed, a.history[first:])
		a.history = trimmed
	}
	a.latest = a.snapshotLocked(time.Now(), false)
	snapshot := cloneSnapshot(a.latest, false)
	a.stateMu.Unlock()

	a.printStatus(snapshot)
	payload, err := json.Marshal(snapshot)
	if err == nil {
		a.publish(payload)
	}
}

func (a *Application) snapshot(now time.Time, includeHistory bool) Snapshot {
	a.stateMu.RLock()
	defer a.stateMu.RUnlock()
	return cloneSnapshot(a.snapshotLocked(now, includeHistory), includeHistory)
}

func (a *Application) snapshotLocked(now time.Time, includeHistory bool) Snapshot {
	snapshot := Snapshot{
		StartedAt:  a.startedAt,
		Generated:  now,
		Version:    a.config.Version,
		IntervalMS: a.config.Interval.Milliseconds(),
		Immediate:  quality.Calculate(a.history, now, a.config.ImmediateWindow, true),
		Stability:  quality.Calculate(a.history, now, a.config.StabilityWindow, false),
	}
	if len(a.history) > 0 {
		latest := cloneSample(a.history[len(a.history)-1])
		snapshot.Latest = &latest
	}
	if includeHistory {
		snapshot.History = cloneHistory(a.history)
	}
	return snapshot
}

func (a *Application) handleState(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, a.snapshot(time.Now(), true))
}

func (a *Application) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming is unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	updates := a.subscribe()
	defer a.unsubscribe(updates)
	if payload, err := json.Marshal(a.snapshot(time.Now(), false)); err == nil {
		writeEvent(w, payload)
		flusher.Flush()
	}

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-a.done:
			return
		case payload := <-updates:
			writeEvent(w, payload)
			flusher.Flush()
		case <-heartbeat.C:
			_, _ = io.WriteString(w, ": keepalive\n\n")
			flusher.Flush()
		}
	}
}

func (a *Application) handleShutdown(w http.ResponseWriter, r *http.Request) {
	provided := r.Header.Get("X-Constatra-Token")
	if subtle.ConstantTimeCompare([]byte(provided), []byte(a.shutdownToken)) != 1 {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "invalid shutdown token"})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "shutting down"})
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
	go func() {
		time.Sleep(150 * time.Millisecond)
		a.Stop()
		a.requestStop()
	}()
}

func (a *Application) subscribe() chan []byte {
	channel := make(chan []byte, 1)
	a.subscribersMu.Lock()
	a.subscribers[channel] = struct{}{}
	a.subscribersMu.Unlock()
	return channel
}

func (a *Application) unsubscribe(channel chan []byte) {
	a.subscribersMu.Lock()
	delete(a.subscribers, channel)
	a.subscribersMu.Unlock()
}

func (a *Application) publish(payload []byte) {
	a.subscribersMu.Lock()
	defer a.subscribersMu.Unlock()
	for channel := range a.subscribers {
		select {
		case channel <- payload:
		default:
			select {
			case <-channel:
			default:
			}
			select {
			case channel <- payload:
			default:
			}
		}
	}
}

func (a *Application) printStatus(snapshot Snapshot) {
	if a.config.Output == nil {
		return
	}
	format := "NOW %-9s %4.0f ms  loss %5.1f%% | STABILITY %-9s %4.0f ms  loss %5.1f%%  available %5.1f%%"
	line := fmt.Sprintf(format,
		snapshot.Immediate.Label, snapshot.Immediate.P90LatencyMS, snapshot.Immediate.LossPct,
		snapshot.Stability.Label, snapshot.Stability.P90LatencyMS, snapshot.Stability.LossPct,
		snapshot.Stability.AvailabilityPct,
	)
	if a.config.Interactive {
		fmt.Fprintf(a.config.Output, "\r%-150s", line)
		return
	}
	fmt.Fprintln(a.config.Output, line)
}

func writeEvent(w io.Writer, payload []byte) {
	_, _ = fmt.Fprintf(w, "event: snapshot\ndata: %s\n\n", payload)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("write JSON response: %v", err)
	}
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'; connect-src 'self'; img-src 'self' data:; style-src 'self'; script-src 'self'; base-uri 'none'; frame-ancestors 'none'; form-action 'none'")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

func cloneSnapshot(snapshot Snapshot, includeHistory bool) Snapshot {
	copy := snapshot
	if snapshot.Latest != nil {
		latest := cloneSample(*snapshot.Latest)
		copy.Latest = &latest
	}
	if includeHistory {
		copy.History = cloneHistory(snapshot.History)
	} else {
		copy.History = nil
	}
	return copy
}

func cloneHistory(history []probe.Sample) []probe.Sample {
	copy := make([]probe.Sample, len(history))
	for index, sample := range history {
		copy[index] = cloneSample(sample)
	}
	return copy
}

func cloneSample(sample probe.Sample) probe.Sample {
	copy := sample
	copy.Results = append([]probe.Result(nil), sample.Results...)
	return copy
}
