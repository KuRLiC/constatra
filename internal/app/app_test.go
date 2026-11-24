package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/KuRLiC/constatra/internal/quality"
)

func TestStateStartsInCheckingMode(t *testing.T) {
	application := newTestApplication(t, func() {})
	request := httptest.NewRequest(http.MethodGet, "/api/state", nil)
	recorder := httptest.NewRecorder()
	application.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	var snapshot Snapshot
	if err := json.Unmarshal(recorder.Body.Bytes(), &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.Immediate.Level != quality.Checking || snapshot.Stability.Level != quality.Checking {
		t.Fatalf("expected checking state, got immediate=%s stability=%s", snapshot.Immediate.Level, snapshot.Stability.Level)
	}
}

func TestShutdownRequiresToken(t *testing.T) {
	application := newTestApplication(t, func() {})
	request := httptest.NewRequest(http.MethodPost, "/api/shutdown", nil)
	recorder := httptest.NewRecorder()
	application.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", recorder.Code)
	}
}

func TestShutdownAcceptsTokenAndRequestsStop(t *testing.T) {
	stopped := make(chan struct{}, 1)
	application := newTestApplication(t, func() { stopped <- struct{}{} })
	request := httptest.NewRequest(http.MethodPost, "/api/shutdown", strings.NewReader(""))
	request.Header.Set("X-Constatra-Token", application.shutdownToken)
	recorder := httptest.NewRecorder()
	application.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", recorder.Code)
	}
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("shutdown callback was not invoked")
	}
	select {
	case <-application.done:
	default:
		t.Fatal("application streams were not stopped")
	}
}

func TestEventStreamStopsWithApplication(t *testing.T) {
	application := newTestApplication(t, func() {})
	server := httptest.NewServer(application.Handler())
	defer server.Close()

	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL+"/api/events", nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	application.Stop()
	_ = response.Body.Close()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		application.subscribersMu.Lock()
		count := len(application.subscribers)
		application.subscribersMu.Unlock()
		if count == 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("SSE subscriber was not removed after stop")
}

func newTestApplication(t *testing.T, stop func()) *Application {
	t.Helper()
	application, err := New(Config{
		Interval:        time.Second,
		ImmediateWindow: 3 * time.Second,
		StabilityWindow: 30 * time.Second,
		HistoryWindow:   15 * time.Minute,
		Targets:         []string{"127.0.0.1"},
		Port:            443,
		Timeout:         100 * time.Millisecond,
		Version:         "test",
	}, stop)
	if err != nil {
		t.Fatal(err)
	}
	return application
}
