package probe

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestEngineMeasuresLocalTCPListener(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		connection, acceptErr := listener.Accept()
		if acceptErr == nil {
			_ = connection.Close()
		}
	}()

	engine := Engine{
		Targets: []string{"127.0.0.1"},
		Port:    listener.Addr().(*net.TCPAddr).Port,
		Timeout: 200 * time.Millisecond,
	}
	sample := engine.Measure(context.Background())
	if len(sample.Results) != 1 || !sample.Results[0].Success {
		t.Fatalf("expected successful local probe, got %#v", sample.Results)
	}
	if sample.LossPct != 0 {
		t.Fatalf("expected 0%% probe loss, got %.1f", sample.LossPct)
	}
}

func TestEngineReportsRefusedConnectionAsProbeLoss(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()

	engine := Engine{Targets: []string{"127.0.0.1"}, Port: port, Timeout: 200 * time.Millisecond}
	sample := engine.Measure(context.Background())
	if sample.Results[0].Success {
		t.Fatal("expected a refused connection to fail")
	}
	if sample.LossPct != 100 {
		t.Fatalf("expected 100%% probe loss, got %.1f", sample.LossPct)
	}
}
