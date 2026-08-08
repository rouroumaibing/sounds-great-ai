package telemetry

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// startMockPrometheus 启动一个返回指定 metrics 文本的测试 HTTP 服务器。
func startMockPrometheus(metricsText string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(metricsText))
	}))
}

const normalMetrics = `# HELP dog_pack_invocation_completed Completed invocations
# TYPE dog_pack_invocation_completed counter
dog_pack_invocation_completed{status="ok"} 100
dog_pack_invocation_completed{status="error"} 5
# HELP dog_pack_invocation_duration_bucket Duration histogram
# TYPE dog_pack_invocation_duration_bucket histogram
dog_pack_invocation_duration_bucket{le="1"} 50
dog_pack_invocation_duration_bucket{le="10"} 90
dog_pack_invocation_duration_bucket{le="60"} 100
dog_pack_invocation_duration_bucket{le="120"} 105
dog_pack_invocation_duration_count 105
# HELP dog_pack_agent_liveness Agent liveness gauge
# TYPE dog_pack_agent_liveness gauge
dog_pack_agent_liveness{breed="bianmu"} 1
dog_pack_agent_liveness{breed="xigou"} 1
`

const highErrorMetrics = `# HELP dog_pack_invocation_completed Completed invocations
# TYPE dog_pack_invocation_completed counter
dog_pack_invocation_completed{status="ok"} 70
dog_pack_invocation_completed{status="error"} 35
# HELP dog_pack_invocation_duration_bucket Duration histogram
# TYPE dog_pack_invocation_duration_bucket histogram
dog_pack_invocation_duration_bucket{le="1"} 50
dog_pack_invocation_duration_bucket{le="10"} 90
dog_pack_invocation_duration_bucket{le="60"} 100
dog_pack_invocation_duration_bucket{le="120"} 105
dog_pack_invocation_duration_count 105
# HELP dog_pack_agent_liveness Agent liveness gauge
# TYPE dog_pack_agent_liveness gauge
dog_pack_agent_liveness{breed="bianmu"} 1
`

func TestReadMetricsNormal(t *testing.T) {
	srv := startMockPrometheus(normalMetrics)
	defer srv.Close()

	cfg := DefaultBurnRateConfig()
	cfg.PrometheusURL = srv.URL
	m := NewBurnRateMonitor(cfg, nil)

	errorRate, p95, active, err := m.readMetrics()
	if err != nil {
		t.Fatalf("readMetrics failed: %v", err)
	}
	// errorRate = 5 / (100 + 5) ≈ 0.0476
	if errorRate > 0.05 {
		t.Errorf("expected errorRate < 0.05, got %f", errorRate)
	}
	if p95 == 0 {
		t.Error("expected non-zero p95")
	}
	if active != 0 {
		t.Errorf("expected 0 active invocations, got %d", active)
	}
}

func TestReadMetricsHighError(t *testing.T) {
	srv := startMockPrometheus(highErrorMetrics)
	defer srv.Close()

	cfg := DefaultBurnRateConfig()
	cfg.PrometheusURL = srv.URL
	m := NewBurnRateMonitor(cfg, nil)

	errorRate, _, _, err := m.readMetrics()
	if err != nil {
		t.Fatalf("readMetrics failed: %v", err)
	}
	// errorRate = 35 / (70 + 35) ≈ 0.333 > 0.3
	if errorRate <= 0.3 {
		t.Errorf("expected errorRate > 0.3, got %f", errorRate)
	}
}

func TestCheckThresholdsNoBreach(t *testing.T) {
	srv := startMockPrometheus(normalMetrics)
	defer srv.Close()

	cfg := DefaultBurnRateConfig()
	cfg.PrometheusURL = srv.URL
	cfg.ConsecutiveBreaches = 3

	var alerts []BurnRateAlert
	var mu sync.Mutex
	m := NewBurnRateMonitor(cfg, func(a BurnRateAlert) {
		mu.Lock()
		alerts = append(alerts, a)
		mu.Unlock()
	})

	// 正常指标 → 不触发告警
	m.checkOnce(nil)

	mu.Lock()
	defer mu.Unlock()
	if len(alerts) != 0 {
		t.Errorf("expected 0 alerts, got %d", len(alerts))
	}
	if m.breaches != 0 {
		t.Errorf("expected 0 breaches, got %d", m.breaches)
	}
}

func TestCheckThresholdsBreachAndAlert(t *testing.T) {
	srv := startMockPrometheus(highErrorMetrics)
	defer srv.Close()

	cfg := DefaultBurnRateConfig()
	cfg.PrometheusURL = srv.URL
	cfg.ConsecutiveBreaches = 3

	var alerts []BurnRateAlert
	var mu sync.Mutex
	m := NewBurnRateMonitor(cfg, func(a BurnRateAlert) {
		mu.Lock()
		alerts = append(alerts, a)
		mu.Unlock()
	})

	// 连续 3 次突破 → 第 3 次触发告警
	m.checkOnce(nil) // breaches=1
	m.checkOnce(nil) // breaches=2
	m.checkOnce(nil) // breaches=3 → alert

	mu.Lock()
	defer mu.Unlock()
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert after 3 breaches, got %d", len(alerts))
	}
	if alerts[0].Type != "error_rate" {
		t.Errorf("expected alert type error_rate, got %s", alerts[0].Type)
	}
}

func TestCheckThresholdsRecovery(t *testing.T) {
	srv := startMockPrometheus(normalMetrics)
	defer srv.Close()
	srvHigh := startMockPrometheus(highErrorMetrics)
	defer srvHigh.Close()

	cfg := DefaultBurnRateConfig()
	cfg.ConsecutiveBreaches = 1 // 快速触发

	var alerts []BurnRateAlert
	var mu sync.Mutex
	m := NewBurnRateMonitor(cfg, func(a BurnRateAlert) {
		mu.Lock()
		alerts = append(alerts, a)
		mu.Unlock()
	})

	// 先用高错误率指标触发告警
	m.config.PrometheusURL = srvHigh.URL
	m.checkOnce(nil) // breaches=1 → alert

	// 切换回正常指标 → 恢复通知
	m.config.PrometheusURL = srv.URL
	m.checkOnce(nil) // recovery

	mu.Lock()
	defer mu.Unlock()
	if len(alerts) != 2 {
		t.Fatalf("expected 2 alerts (breach + recovery), got %d", len(alerts))
	}
	if alerts[1].Type != "recovery" {
		t.Errorf("expected recovery alert, got type %s", alerts[1].Type)
	}
}

func TestCheckThresholdsNoDuplicateAlert(t *testing.T) {
	srv := startMockPrometheus(highErrorMetrics)
	defer srv.Close()

	cfg := DefaultBurnRateConfig()
	cfg.PrometheusURL = srv.URL
	cfg.ConsecutiveBreaches = 1

	var alerts []BurnRateAlert
	var mu sync.Mutex
	m := NewBurnRateMonitor(cfg, func(a BurnRateAlert) {
		mu.Lock()
		alerts = append(alerts, a)
		mu.Unlock()
	})

	// 连续 5 次突破 → 只触发 1 次告警（不重复）
	for i := 0; i < 5; i++ {
		m.checkOnce(nil)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(alerts) != 1 {
		t.Errorf("expected 1 alert (no duplicate), got %d", len(alerts))
	}
}
