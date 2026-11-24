(() => {
  "use strict";

  const LEVELS = ["checking", "excellent", "good", "poor", "critical", "offline"];
  const MAX_HISTORY_MS = 15 * 60 * 1000;
  const state = {
    history: [],
    rangeMinutes: 5,
    startedAt: null,
    intervalMs: 1000,
    source: null,
    reconnectTimer: null,
    stopped: false,
  };

  const elements = {
    liveBadge: document.querySelector("#live-badge"),
    immediateCard: document.querySelector("#immediate-card"),
    immediateLabel: document.querySelector("#immediate-label"),
    immediateLatency: document.querySelector("#immediate-latency"),
    immediateLoss: document.querySelector("#immediate-loss"),
    immediateJitter: document.querySelector("#immediate-jitter"),
    immediateScore: document.querySelector("#immediate-score"),
    stabilityCard: document.querySelector("#stability-card"),
    stabilityLabel: document.querySelector("#stability-label"),
    stabilityAvailability: document.querySelector("#stability-availability"),
    stabilityLoss: document.querySelector("#stability-loss"),
    stabilityLatency: document.querySelector("#stability-latency"),
    stabilityScore: document.querySelector("#stability-score"),
    latestLatency: document.querySelector("#latest-latency"),
    latestLoss: document.querySelector("#latest-loss"),
    uptime: document.querySelector("#uptime"),
    interval: document.querySelector("#interval"),
    appVersion: document.querySelector("#app-version"),
    targets: document.querySelector("#targets"),
    chart: document.querySelector("#history-chart"),
    fullscreenButton: document.querySelector("#fullscreen-button"),
    shutdownButton: document.querySelector("#shutdown-button"),
    toast: document.querySelector("#toast"),
  };

  function finite(value, fallback = 0) {
    const number = Number(value);
    return Number.isFinite(number) ? number : fallback;
  }

  function timeOf(sample) {
    const parsed = Date.parse(sample?.time);
    return Number.isFinite(parsed) ? parsed : 0;
  }

  function formatNumber(value, digits = 0) {
    const number = Number(value);
    return Number.isFinite(number) ? number.toFixed(digits) : "—";
  }

  function formatMS(value) {
    const number = Number(value);
    if (!Number.isFinite(number) || number <= 0) return "—";
    return number < 10 ? `${number.toFixed(1)} ms` : `${Math.round(number)} ms`;
  }

  function formatPercent(value) {
    const number = Number(value);
    if (!Number.isFinite(number)) return "—";
    const digits = number > 0 && number < 10 ? 1 : 0;
    return `${number.toFixed(digits)}%`;
  }

  function setLevel(card, level) {
    const normalized = LEVELS.includes(level) ? level : "checking";
    card.classList.remove(...LEVELS);
    card.classList.add(normalized);
  }

  function renderQuality(prefix, stats) {
    if (!stats) return;
    const level = String(stats.level || "checking").toLowerCase();
    const label = String(stats.label || level).toUpperCase();

    if (prefix === "immediate") {
      setLevel(elements.immediateCard, level);
      elements.immediateLabel.textContent = label;
      elements.immediateLatency.textContent = formatNumber(stats.p90LatencyMs);
      elements.immediateLoss.textContent = formatPercent(stats.lossPct);
      elements.immediateJitter.textContent = formatMS(stats.jitterMs);
      elements.immediateScore.textContent = Number.isFinite(Number(stats.score)) ? `${Math.round(stats.score)}/100` : "—";
      return;
    }

    setLevel(elements.stabilityCard, level);
    elements.stabilityLabel.textContent = label;
    elements.stabilityAvailability.textContent = formatNumber(stats.availabilityPct);
    elements.stabilityLoss.textContent = formatPercent(stats.lossPct);
    elements.stabilityLatency.textContent = formatMS(stats.p90LatencyMs);
    elements.stabilityScore.textContent = Number.isFinite(Number(stats.score)) ? `${Math.round(stats.score)}/100` : "—";
  }

  function renderLatest(sample) {
    if (!sample) return;
    elements.latestLatency.textContent = formatMS(sample.latencyMs);
    elements.latestLoss.textContent = formatPercent(sample.lossPct);
    elements.targets.replaceChildren();

    for (const result of sample.results || []) {
      const item = document.createElement("span");
      item.className = `target${result.success ? " ok" : ""}`;
      const dot = document.createElement("i");
      const target = document.createElement("span");
      const reading = document.createElement("strong");
      target.textContent = result.target || "Unknown target";
      reading.textContent = result.success ? formatMS(result.latencyMs) : String(result.error || "missed");
      item.append(dot, target, reading);
      elements.targets.append(item);
    }
  }

  function mergeHistory(samples) {
    if (!Array.isArray(samples)) return;
    const byTime = new Map(state.history.map((sample) => [timeOf(sample), sample]));
    for (const sample of samples) {
      const time = timeOf(sample);
      if (time) byTime.set(time, sample);
    }
    const cutoff = Date.now() - MAX_HISTORY_MS;
    state.history = [...byTime.values()]
      .filter((sample) => timeOf(sample) >= cutoff)
      .sort((a, b) => timeOf(a) - timeOf(b));
  }

  function applySnapshot(snapshot, initial = false) {
    if (!snapshot || typeof snapshot !== "object") return;
    state.startedAt = snapshot.startedAt || state.startedAt;
    state.intervalMs = finite(snapshot.intervalMs, state.intervalMs);
    elements.appVersion.textContent = snapshot.version || "—";
    elements.interval.textContent = state.intervalMs >= 1000
      ? `${(state.intervalMs / 1000).toFixed(state.intervalMs % 1000 ? 1 : 0)} s`
      : `${Math.round(state.intervalMs)} ms`;

    renderQuality("immediate", snapshot.immediate);
    renderQuality("stability", snapshot.stability);

    const latest = snapshot.latest || snapshot.latestSample || null;
    if (initial) mergeHistory(snapshot.history || []);
    if (latest) {
      mergeHistory([latest]);
      renderLatest(latest);
    }
    drawChart();
  }

  function setConnectionStatus(mode, text) {
    elements.liveBadge.classList.toggle("waiting", mode === "waiting");
    elements.liveBadge.classList.toggle("lost", mode === "lost");
    elements.liveBadge.querySelector("span").textContent = text;
  }

  async function loadInitialState() {
    try {
      const response = await fetch("/api/state", { cache: "no-store" });
      if (!response.ok) throw new Error(`HTTP ${response.status}`);
      applySnapshot(await response.json(), true);
    } catch (error) {
      showToast(`Could not load initial history: ${error.message}`);
    }
  }

  function connectEvents() {
    if (state.stopped) return;
    if (state.source) state.source.close();
    setConnectionStatus("waiting", "CONNECTING");

    const source = new EventSource("/api/events");
    state.source = source;

    source.addEventListener("open", () => setConnectionStatus("live", "LIVE"));
    const receive = (event) => {
      try {
        applySnapshot(JSON.parse(event.data));
        setConnectionStatus("live", "LIVE");
      } catch {
        showToast("Received an invalid live update");
      }
    };
    source.addEventListener("message", receive);
    source.addEventListener("snapshot", receive);
    source.addEventListener("error", () => {
      source.close();
      if (state.source === source) state.source = null;
      if (state.stopped) return;
      setConnectionStatus("lost", "RECONNECTING");
      clearTimeout(state.reconnectTimer);
      state.reconnectTimer = setTimeout(connectEvents, 1500);
    });
  }

  function chartSamples() {
    const cutoff = Date.now() - state.rangeMinutes * 60 * 1000;
    return state.history.filter((sample) => timeOf(sample) >= cutoff);
  }

  function drawChart() {
    const canvas = elements.chart;
    const context = canvas.getContext("2d");
    const rect = canvas.getBoundingClientRect();
    if (!rect.width || !rect.height) return;

    const ratio = Math.max(1, window.devicePixelRatio || 1);
    const pixelWidth = Math.round(rect.width * ratio);
    const pixelHeight = Math.round(rect.height * ratio);
    if (canvas.width !== pixelWidth || canvas.height !== pixelHeight) {
      canvas.width = pixelWidth;
      canvas.height = pixelHeight;
    }
    context.setTransform(ratio, 0, 0, ratio, 0, 0);
    context.clearRect(0, 0, rect.width, rect.height);

    const width = rect.width;
    const height = rect.height;
    const padding = { top: 12, right: 14, bottom: 24, left: 48 };
    const plotWidth = Math.max(1, width - padding.left - padding.right);
    const plotHeight = Math.max(1, height - padding.top - padding.bottom);
    const samples = chartSamples();
    const now = Date.now();
    const start = now - state.rangeMinutes * 60 * 1000;
    const validLatencies = samples.map((sample) => finite(sample.latencyMs)).filter((value) => value > 0);
    const highest = validLatencies.length ? Math.max(...validLatencies) : 0;
    const maxLatency = Math.max(100, Math.ceil(highest / 50) * 50);

    context.font = "11px system-ui, sans-serif";
    context.lineWidth = 1;
    context.textBaseline = "middle";
    for (let line = 0; line <= 4; line++) {
      const y = padding.top + (plotHeight * line) / 4;
      const value = maxLatency * (1 - line / 4);
      context.strokeStyle = "rgba(71, 88, 112, .35)";
      context.beginPath();
      context.moveTo(padding.left, y);
      context.lineTo(width - padding.right, y);
      context.stroke();
      context.fillStyle = "#7f8da1";
      context.textAlign = "right";
      context.fillText(`${Math.round(value)} ms`, padding.left - 7, y);
    }

    context.textBaseline = "alphabetic";
    context.fillStyle = "#7f8da1";
    context.textAlign = "left";
    context.fillText(`-${state.rangeMinutes}m`, padding.left, height - 3);
    context.textAlign = "right";
    context.fillText("now", width - padding.right, height - 3);

    if (samples.length === 0) {
      context.fillStyle = "#8e9bad";
      context.textAlign = "center";
      context.textBaseline = "middle";
      context.fillText("Waiting for connection measurements…", padding.left + plotWidth / 2, padding.top + plotHeight / 2);
      return;
    }

    const xFor = (sample) => padding.left + ((timeOf(sample) - start) / (now - start)) * plotWidth;
    const yFor = (latency) => padding.top + (1 - Math.min(latency, maxLatency) / maxLatency) * plotHeight;

    // Missed probes are deliberately prominent but occupy only the lower third.
    const barWidth = Math.max(2, Math.min(10, plotWidth / Math.max(samples.length, 1) * 0.72));
    for (const sample of samples) {
      const loss = Math.max(0, Math.min(100, finite(sample.lossPct)));
      if (loss <= 0) continue;
      const barHeight = Math.max(4, (loss / 100) * plotHeight * 0.35);
      context.fillStyle = loss >= 99 ? "rgba(240, 68, 85, .9)" : "rgba(240, 68, 85, .55)";
      context.fillRect(xFor(sample) - barWidth / 2, padding.top + plotHeight - barHeight, barWidth, barHeight);
    }

    const gradient = context.createLinearGradient(padding.left, 0, width - padding.right, 0);
    gradient.addColorStop(0, "#25d366");
    gradient.addColorStop(0.65, "#f5cf3a");
    gradient.addColorStop(1, "#ff922b");
    context.strokeStyle = gradient;
    context.lineWidth = 2.5;
    context.lineJoin = "round";
    context.lineCap = "round";
    context.shadowColor = "rgba(37, 211, 102, .35)";
    context.shadowBlur = 7;
    context.beginPath();
    let drawing = false;
    for (const sample of samples) {
      const latency = finite(sample.latencyMs);
      if (latency <= 0 || finite(sample.lossPct) >= 100) {
        drawing = false;
        continue;
      }
      const x = xFor(sample);
      const y = yFor(latency);
      if (!drawing) context.moveTo(x, y);
      else context.lineTo(x, y);
      drawing = true;
    }
    context.stroke();
    context.shadowBlur = 0;
  }

  function updateUptime() {
    if (!state.startedAt) {
      elements.uptime.textContent = "—";
      return;
    }
    const seconds = Math.max(0, Math.floor((Date.now() - Date.parse(state.startedAt)) / 1000));
    if (!Number.isFinite(seconds)) return;
    const hours = Math.floor(seconds / 3600);
    const minutes = Math.floor((seconds % 3600) / 60);
    const remainder = seconds % 60;
    elements.uptime.textContent = hours > 0
      ? `${hours}h ${String(minutes).padStart(2, "0")}m`
      : `${minutes}m ${String(remainder).padStart(2, "0")}s`;
  }

  let toastTimer;
  function showToast(message) {
    elements.toast.textContent = message;
    elements.toast.classList.add("show");
    clearTimeout(toastTimer);
    toastTimer = setTimeout(() => elements.toast.classList.remove("show"), 3500);
  }

  async function stopApplication() {
    if (!window.confirm("Stop Constatra and close its local server?")) return;
    elements.shutdownButton.disabled = true;
    elements.shutdownButton.textContent = "Stopping…";
    const token = document.querySelector('meta[name="shutdown-token"]')?.content || "";
    try {
      const response = await fetch("/api/shutdown", {
        method: "POST",
        headers: { "X-Constatra-Token": token },
      });
      if (!response.ok) throw new Error(`HTTP ${response.status}`);
      state.stopped = true;
      if (state.source) state.source.close();
      setConnectionStatus("waiting", "STOPPED");
      showToast("Constatra stopped. You can close this tab.");
    } catch (error) {
      elements.shutdownButton.disabled = false;
      elements.shutdownButton.textContent = "Stop";
      showToast(`Could not stop Constatra: ${error.message}`);
    }
  }

  async function toggleFullscreen() {
    try {
      if (!document.fullscreenElement) await document.documentElement.requestFullscreen();
      else await document.exitFullscreen();
    } catch {
      showToast("Fullscreen mode is not available in this browser");
    }
  }

  document.querySelectorAll(".range-picker button").forEach((button) => {
    button.addEventListener("click", () => {
      state.rangeMinutes = finite(button.dataset.minutes, 5);
      document.querySelectorAll(".range-picker button").forEach((item) => item.classList.toggle("active", item === button));
      drawChart();
    });
  });
  elements.shutdownButton.addEventListener("click", stopApplication);
  elements.fullscreenButton.addEventListener("click", toggleFullscreen);
  document.addEventListener("fullscreenchange", () => {
    elements.fullscreenButton.textContent = document.fullscreenElement ? "Exit fullscreen" : "Fullscreen";
  });
  window.addEventListener("resize", drawChart);
  if ("ResizeObserver" in window) new ResizeObserver(drawChart).observe(elements.chart);

  setInterval(updateUptime, 1000);
  setInterval(drawChart, 5000);
  updateUptime();
  loadInitialState().finally(connectEvents);
})();
