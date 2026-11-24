package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/KuRLiC/constatra/internal/app"
)

var version = "dev"

type options struct {
	host             string
	port             int
	targets          string
	targetPort       int
	interval         time.Duration
	timeout          time.Duration
	immediateWindow  time.Duration
	stabilityWindow  time.Duration
	historyWindow    time.Duration
	disableBrowser   bool
	printVersionOnly bool
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "constatra: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	options := parseFlags()
	if options.printVersionOnly {
		fmt.Printf("Constatra %s\n", version)
		return nil
	}
	targets, err := validate(options)
	if err != nil {
		return err
	}

	listener, err := net.Listen("tcp", net.JoinHostPort(options.host, strconv.Itoa(options.port)))
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	defer listener.Close()
	address := listener.Addr().(*net.TCPAddr)
	url := "http://" + net.JoinHostPort(options.host, strconv.Itoa(address.Port))

	signalContext, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()
	ctx, cancel := context.WithCancel(signalContext)
	defer cancel()

	application, err := app.New(app.Config{
		Interval:        options.interval,
		ImmediateWindow: options.immediateWindow,
		StabilityWindow: options.stabilityWindow,
		HistoryWindow:   options.historyWindow,
		Targets:         targets,
		Port:            options.targetPort,
		Timeout:         options.timeout,
		Version:         version,
		Output:          os.Stdout,
		Interactive:     isTerminal(os.Stdout),
	}, cancel)
	if err != nil {
		return err
	}

	server := &http.Server{
		Handler:           application.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	serveErrors := make(chan error, 1)
	go func() {
		serveErrors <- server.Serve(listener)
	}()
	fmt.Printf("Constatra %s is running\n", version)
	fmt.Printf("Dashboard: %s\n", url)
	fmt.Printf("Probing %s:%d every %s (timeout %s)\n", strings.Join(targets, ", "), options.targetPort, options.interval, options.timeout)
	fmt.Println("Press Ctrl-C or use the Stop button in the dashboard to exit.")
	if !options.disableBrowser {
		if err := openBrowser(url); err != nil {
			fmt.Printf("Could not open the browser automatically: %v\n", err)
			fmt.Printf("Open this address manually: %s\n", url)
		}
	}
	go application.RunMeasurements(ctx)

	select {
	case <-ctx.Done():
	case err := <-serveErrors:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			cancel()
			return fmt.Errorf("serve dashboard: %w", err)
		}
	}

	application.Stop()
	shutdownContext, shutdownCancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownContext); err != nil {
		return fmt.Errorf("shut down server: %w", err)
	}
	fmt.Println("Constatra stopped cleanly.")
	return nil
}

func parseFlags() options {
	var options options
	flag.StringVar(&options.host, "host", "127.0.0.1", "local address used by the dashboard")
	flag.IntVar(&options.port, "port", 0, "dashboard port; 0 chooses a free random port")
	flag.StringVar(&options.targets, "targets", "1.1.1.1,8.8.8.8,9.9.9.9", "comma-separated probe targets")
	flag.IntVar(&options.targetPort, "probe-port", 443, "TCP port used by connection probes")
	flag.DurationVar(&options.interval, "interval", time.Second, "time between measurements")
	flag.DurationVar(&options.timeout, "timeout", 800*time.Millisecond, "timeout for each parallel probe")
	flag.DurationVar(&options.immediateWindow, "immediate-window", 3*time.Second, "rolling window for RIGHT NOW")
	flag.DurationVar(&options.stabilityWindow, "stability-window", 30*time.Second, "rolling window for STABILITY")
	flag.DurationVar(&options.historyWindow, "history", 15*time.Minute, "amount of chart history retained in memory")
	flag.BoolVar(&options.disableBrowser, "no-browser", false, "do not open the dashboard automatically")
	flag.BoolVar(&options.printVersionOnly, "version", false, "print the version and exit")
	flag.Parse()
	return options
}

func validate(options options) ([]string, error) {
	if net.ParseIP(options.host) == nil || !net.ParseIP(options.host).IsLoopback() {
		return nil, errors.New("--host must be a loopback IP address")
	}
	if options.port < 0 || options.port > 65535 {
		return nil, errors.New("--port must be between 0 and 65535")
	}
	if options.targetPort < 1 || options.targetPort > 65535 {
		return nil, errors.New("--probe-port must be between 1 and 65535")
	}
	if options.interval < 250*time.Millisecond {
		return nil, errors.New("--interval must be at least 250ms")
	}
	if options.timeout <= 0 || options.timeout >= options.interval {
		return nil, errors.New("--timeout must be positive and shorter than --interval")
	}
	if options.immediateWindow < options.interval {
		return nil, errors.New("--immediate-window must be at least as long as --interval")
	}
	if options.stabilityWindow < options.immediateWindow {
		return nil, errors.New("--stability-window must not be shorter than --immediate-window")
	}
	if options.historyWindow < options.stabilityWindow {
		return nil, errors.New("--history-window must not be shorter than --stability-window")
	}

	parts := strings.Split(options.targets, ",")
	targets := make([]string, 0, len(parts))
	seen := make(map[string]bool)
	for _, part := range parts {
		target := strings.TrimSpace(part)
		if target == "" || seen[target] {
			continue
		}
		seen[target] = true
		targets = append(targets, target)
	}
	if len(targets) == 0 {
		return nil, errors.New("--targets must contain at least one host or IP address")
	}
	return targets, nil
}

func isTerminal(file *os.File) bool {
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func openBrowser(url string) error {
	var command *exec.Cmd
	switch {
	case isWSL():
		path, err := exec.LookPath("cmd.exe")
		if err != nil {
			path = "/mnt/c/Windows/System32/cmd.exe"
			if _, statErr := os.Stat(path); statErr != nil {
				return errors.New("Windows browser launcher cmd.exe was not found")
			}
		}
		command = exec.Command(path, "/c", "start", "", url)
	case runtime.GOOS == "windows":
		command = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case runtime.GOOS == "darwin":
		command = exec.Command("open", url)
	default:
		command = exec.Command("xdg-open", url)
	}
	if err := command.Start(); err != nil {
		return err
	}
	go func() { _ = command.Wait() }()
	return nil
}

func isWSL() bool {
	if os.Getenv("WSL_INTEROP") != "" || os.Getenv("WSL_DISTRO_NAME") != "" {
		return true
	}
	data, err := os.ReadFile("/proc/sys/kernel/osrelease")
	return err == nil && strings.Contains(strings.ToLower(string(data)), "microsoft")
}
