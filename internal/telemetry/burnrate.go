package telemetry

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// BurnRateConfig 配置 burn-rate 监控阈值。
type BurnRateConfig struct {
	ErrorRateThreshold   float64       // 0.3 (30%)
	P95LatencyThreshold  time.Duration // 120s
	ActiveInvocationsMax int           // 50
	CheckInterval        time.Duration // 60s
	ConsecutiveBreaches  int           // 3（连续突破 N 次后触发告警）
	PrometheusURL        string        // http://localhost:9464/metrics
}

// DefaultBurnRateConfig 返回默认配置。
func DefaultBurnRateConfig() BurnRateConfig {
	return BurnRateConfig{
		ErrorRateThreshold:   0.3,
		P95LatencyThreshold:  120 * time.Second,
		ActiveInvocationsMax: 50,
		CheckInterval:        60 * time.Second,
		ConsecutiveBreaches:  3,
		PrometheusURL:        "http://localhost:9464/metrics",
	}
}

// BurnRateAlert 是一次告警事件。
type BurnRateAlert struct {
	Type      string    // "error_rate", "p95_latency", "active_invocations"
	Value     float64
	Threshold float64
	Message   string
	Time      time.Time
}

// BurnRateMonitor 定时检查 Prometheus 指标，超阈值时通过 alertFn 回调推送告警。
type BurnRateMonitor struct {
	config   BurnRateConfig
	mu       sync.Mutex
	breaches int
	alerted  bool // 是否已处于告警状态（避免重复告警）
	alertFn  func(alert BurnRateAlert)
	client   *http.Client
	stopCh   chan struct{}
}

// NewBurnRateMonitor 创建监控器。
func NewBurnRateMonitor(config BurnRateConfig, alertFn func(alert BurnRateAlert)) *BurnRateMonitor {
	return &BurnRateMonitor{
		config:  config,
		alertFn: alertFn,
		client:  &http.Client{Timeout: 10 * time.Second},
		stopCh:  make(chan struct{}),
	}
}

// Start 启动后台 goroutine，每 CheckInterval 检查一次指标。
func (m *BurnRateMonitor) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(m.config.CheckInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-m.stopCh:
				return
			case <-ticker.C:
				m.checkOnce(ctx)
			}
		}
	}()
}

// Stop 停止监控。
func (m *BurnRateMonitor) Stop() {
	close(m.stopCh)
}

// readMetrics 从 Prometheus exporter 抓取指标，解析 Prometheus text format。
// 返回 errorRate、p95 latency、active invocations。
func (m *BurnRateMonitor) readMetrics() (errorRate float64, p95 time.Duration, active int, err error) {
	resp, err := m.client.Get(m.config.PrometheusURL)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("scrape prometheus: %w", err)
	}
	defer resp.Body.Close()

	var okCount, errCount float64
	var histogramBuckets = make(map[float64]float64) // bucket upper bound → cumulative count
	var totalCount float64
	var activeCount int

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		// 解析 prometheus text format: metric_name{labels} value
		metricName, labels, value, parseErr := parsePromLine(line)
		if parseErr != nil {
			continue
		}
		switch metricName {
		case "dog_pack_invocation_completed":
			if labels["status"] == "error" {
				errCount = value
			} else {
				okCount = value
			}
		case "dog_pack_invocation_duration_bucket":
			if leStr, ok := labels["le"]; ok {
				le, e := strconv.ParseFloat(leStr, 64)
				if e == nil {
					histogramBuckets[le] = value
				}
			}
		case "dog_pack_invocation_duration_count":
			totalCount = value
		case "dog_pack_agent_liveness":
			if value >= 2 {
				activeCount++
			}
		}
	}
	if err = scanner.Err(); err != nil {
		return 0, 0, 0, fmt.Errorf("read prometheus body: %w", err)
	}

	total := okCount + errCount
	if total > 0 {
		errorRate = errCount / total
	}
	p95 = calculateP95(histogramBuckets, totalCount, 0.95)
	active = activeCount
	return
}

// parsePromLine 解析一行 Prometheus text format: metric_name{label="val"} value
func parsePromLine(line string) (name string, labels map[string]string, value float64, err error) {
	labels = make(map[string]string)
	braceIdx := strings.Index(line, "{")
	spaceIdx := strings.Index(line, " ")
	if braceIdx == -1 {
		// 无 label 的 metric: metric_name value
		if spaceIdx == -1 {
			return "", nil, 0, fmt.Errorf("no value")
		}
		name = line[:spaceIdx]
		valStr := strings.TrimSpace(line[spaceIdx:])
		value, err = strconv.ParseFloat(valStr, 64)
		return name, labels, value, err
	}
	name = line[:braceIdx]
	closeIdx := strings.Index(line, "}")
	if closeIdx == -1 {
		return "", nil, 0, fmt.Errorf("unclosed labels")
	}
	labelStr := line[braceIdx+1 : closeIdx]
	for _, pair := range strings.Split(labelStr, ",") {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) != 2 {
			continue
		}
		labels[strings.TrimSpace(kv[0])] = strings.Trim(kv[1], `"`)
	}
	valStr := strings.TrimSpace(line[closeIdx+1:])
	value, err = strconv.ParseFloat(valStr, 64)
	return name, labels, value, err
}

// calculateP95 从 histogram buckets 计算 P95 latency。
func calculateP95(buckets map[float64]float64, totalCount float64, quantile float64) time.Duration {
	if totalCount == 0 || len(buckets) == 0 {
		return 0
	}
	target := totalCount * quantile
	// 按 bucket upper bound 排序
	var bounds []float64
	for b := range buckets {
		bounds = append(bounds, b)
	}
	// 简单排序
	for i := 0; i < len(bounds); i++ {
		for j := i + 1; j < len(bounds); j++ {
			if bounds[j] < bounds[i] {
				bounds[i], bounds[j] = bounds[j], bounds[i]
			}
		}
	}
	for _, b := range bounds {
		if buckets[b] >= target {
			return time.Duration(b * float64(time.Second))
		}
	}
	return time.Duration(bounds[len(bounds)-1] * float64(time.Second))
}

// checkOnce 执行一次指标检查。
func (m *BurnRateMonitor) checkOnce(ctx context.Context) {
	errorRate, p95, active, err := m.readMetrics()
	if err != nil {
		log.Printf("BurnRateMonitor: read metrics failed: %v", err)
		return
	}

	var breachAlert *BurnRateAlert
	if errorRate > m.config.ErrorRateThreshold {
		breachAlert = &BurnRateAlert{
			Type:      "error_rate",
			Value:     errorRate,
			Threshold: m.config.ErrorRateThreshold,
			Message:   fmt.Sprintf("当前错误率 %.1f%%，阈值 %.1f%%", errorRate*100, m.config.ErrorRateThreshold*100),
			Time:      time.Now(),
		}
	} else if p95 > m.config.P95LatencyThreshold {
		breachAlert = &BurnRateAlert{
			Type:      "p95_latency",
			Value:     p95.Seconds(),
			Threshold: m.config.P95LatencyThreshold.Seconds(),
			Message:   fmt.Sprintf("当前 P95 延迟 %.1fs，阈值 %.1fs", p95.Seconds(), m.config.P95LatencyThreshold.Seconds()),
			Time:      time.Now(),
		}
	} else if active > m.config.ActiveInvocationsMax {
		breachAlert = &BurnRateAlert{
			Type:      "active_invocations",
			Value:     float64(active),
			Threshold: float64(m.config.ActiveInvocationsMax),
			Message:   fmt.Sprintf("当前活跃调用 %d，阈值 %d", active, m.config.ActiveInvocationsMax),
			Time:      time.Now(),
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if breachAlert != nil {
		m.breaches++
		if m.breaches >= m.config.ConsecutiveBreaches && !m.alerted {
			m.alerted = true
			if m.alertFn != nil {
				m.alertFn(*breachAlert)
			}
		}
	} else {
		// 所有指标正常
		if m.alerted {
			// 发送恢复通知
			m.alerted = false
			if m.alertFn != nil {
				m.alertFn(BurnRateAlert{
					Type:    "recovery",
					Message: "所有指标已恢复正常",
					Time:    time.Now(),
				})
			}
		}
		m.breaches = 0
	}
}
