package main

import (
	"testing"
	"time"
)

func TestValidateAcceptsZeroTuningDefaults(t *testing.T) {
	targets, err := validate(options{
		host:            "127.0.0.1",
		port:            0,
		targets:         "1.1.1.1,8.8.8.8,9.9.9.9",
		targetPort:      443,
		interval:        time.Second,
		timeout:         800 * time.Millisecond,
		immediateWindow: 3 * time.Second,
		stabilityWindow: 30 * time.Second,
		historyWindow:   15 * time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 3 {
		t.Fatalf("expected three targets, got %d", len(targets))
	}
}

func TestValidateRejectsPublicDashboardBinding(t *testing.T) {
	_, err := validate(options{host: "0.0.0.0"})
	if err == nil {
		t.Fatal("expected public bind address to be rejected")
	}
}
