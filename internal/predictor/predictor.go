package predictor

import (
    "fmt"
    "math"
    "k8s-healer/internal/collector"
)

type Predictor struct {
    podHistory  map[string][]collector.PodMetrics
    nodeHistory map[string][]collector.NodeMetrics
}

type PredictionResult struct {
    PodName         string
    PodNamespace    string
    Risk            string
    Issues          []string
    Action          string
    Confidence      int
    Score           float64
    TimeToFailure   string
    Trend           string
    MemoryLeakRate  float64
    CPUGrowthRate   float64
    PredictionHours int
}

type TrendAnalysis struct {
    CPUTrend      string
    MemTrend      string
    CPUSlope      float64
    MemSlope      float64
    IsMemoryLeak  bool
    IsCPUGrowing  bool
    HoursToFailure float64
}

func New() *Predictor {
    return &Predictor{
        podHistory:  make(map[string][]collector.PodMetrics),
        nodeHistory: make(map[string][]collector.NodeMetrics),
    }
}

func (p *Predictor) UpdateHistory(metrics []collector.PodMetrics) {
    for _, metric := range metrics {
        key := fmt.Sprintf("%s/%s", metric.Namespace, metric.Name)
        
        if p.podHistory[key] == nil {
            p.podHistory[key] = make([]collector.PodMetrics, 0)
        }
        
        p.podHistory[key] = append(p.podHistory[key], metric)
        
        // Keep last 20 measurements (10 minutes of history at 30s intervals)
        if len(p.podHistory[key]) > 20 {
            p.podHistory[key] = p.podHistory[key][1:]
        }
    }
}

func (p *Predictor) PredictIssues(currentMetrics []collector.PodMetrics) []PredictionResult {
    var predictions []PredictionResult
    
    for _, metric := range currentMetrics {
        key := fmt.Sprintf("%s/%s", metric.Namespace, metric.Name)
        history := p.podHistory[key]
        
        result := p.analyzePodAdvanced(metric, history)
        
        // Report issues with score > 30 OR predictions with time to failure
        if result.Score > 30 || result.TimeToFailure != "N/A" {
            predictions = append(predictions, result)
        }
    }
    
    return predictions
}

func (p *Predictor) analyzePodAdvanced(current collector.PodMetrics, history []collector.PodMetrics) PredictionResult {
    result := PredictionResult{
        PodName:         current.Name,
        PodNamespace:    current.Namespace,
        Risk:            "LOW",
        Issues:          []string{},
        Action:          "MONITOR",
        Confidence:      100,
        Score:           0,
        TimeToFailure:   "N/A",
        Trend:           "STABLE",
        MemoryLeakRate:  0,
        CPUGrowthRate:   0,
        PredictionHours: 0,
    }
    
    score := 0.0
    
    // === 1. CURRENT RESOURCE THRESHOLDS ===
    if current.CPUPercent > 15 {
        result.Issues = append(result.Issues, fmt.Sprintf("CRITICAL CPU: %.1f%%", current.CPUPercent))
        result.Risk = "CRITICAL"
        result.Action = "SCALE_UP_URGENT"
        score += 40
    } else if current.CPUPercent > 10 {
        result.Issues = append(result.Issues, fmt.Sprintf("HIGH CPU: %.1f%%", current.CPUPercent))
        result.Risk = "HIGH"
        result.Action = "SCALE_UP"
        score += 25
    }
    
    if current.MemPercent > 15 {
        result.Issues = append(result.Issues, fmt.Sprintf("CRITICAL Memory: %.1f%%", current.MemPercent))
        result.Risk = "CRITICAL"
        result.Action = "RESTART_POD_URGENT"
        score += 40
    } else if current.MemPercent > 10 {
        result.Issues = append(result.Issues, fmt.Sprintf("HIGH Memory: %.1f%%", current.MemPercent))
        result.Risk = "HIGH"
        result.Action = "RESTART_POD"
        score += 25
    }
    
    // === 2. TREND ANALYSIS & 24-72 HOUR PREDICTIONS ===
    if len(history) >= 5 {
        trend := p.calculateAdvancedTrend(history, current)
        result.MemoryLeakRate = trend.MemSlope
        result.CPUGrowthRate = trend.CPUSlope
        
        // CPU Growth Prediction (24-72 hour window)
        if trend.CPUSlope > 2 { // Growing >2% per hour
            hoursToFailure := (100 - current.CPUPercent) / trend.CPUSlope
            if hoursToFailure > 0 && hoursToFailure <= 72 {
                result.Issues = append(result.Issues, 
                    fmt.Sprintf("🔮 CPU PREDICTION: Growing %.1f%%/hour → will reach 100%% in %.1f hours", 
                        trend.CPUSlope, hoursToFailure))
                result.TimeToFailure = fmt.Sprintf("%.1f hours (CPU overload)", hoursToFailure)
                result.PredictionHours = int(hoursToFailure)
                score += 30
                
                if hoursToFailure < 24 {
                    result.Risk = "CRITICAL"
                    result.Action = "SCALE_UP_URGENT"
                    score += 20
                } else {
                    result.Risk = "HIGH"
                    result.Action = "SCALE_UP_PLANNED"
                }
            }
        }
        
        // Memory Leak Detection (most important!)
        if trend.MemSlope > 1 { // Growing >1% per hour
            hoursToFailure := (100 - current.MemPercent) / trend.MemSlope
            if hoursToFailure > 0 && hoursToFailure <= 72 {
                result.Issues = append(result.Issues, 
                    fmt.Sprintf("🚨 MEMORY LEAK DETECTED: Growing %.1f%%/hour → OOM in %.1f hours", 
                        trend.MemSlope, hoursToFailure))
                result.TimeToFailure = fmt.Sprintf("%.1f hours (Memory leak)", hoursToFailure)
                result.PredictionHours = int(hoursToFailure)
                score += 35
                
                if hoursToFailure < 12 {
                    result.Risk = "CRITICAL"
                    result.Action = "RESTART_POD_URGENT"
                    result.Issues = append(result.Issues, "IMMEDIATE ACTION REQUIRED")
                    score += 25
                } else if hoursToFailure < 24 {
                    result.Risk = "HIGH" 
                    result.Action = "RESTART_POD_PLANNED"
                } else {
                    result.Risk = "MEDIUM"
                    result.Action = "MONITOR_MEMORY_LEAK"
                }
            }
        }
        
        // Performance Degradation Detection
        if p.detectPerformanceDegradation(history, current) {
            result.Issues = append(result.Issues, "Performance degradation detected over time")
            score += 20
            if result.Risk == "LOW" {
                result.Risk = "MEDIUM"
                result.Action = "INVESTIGATE_PERFORMANCE"
            }
        }
        
        // Set trend description
        if trend.CPUSlope > 2 && trend.MemSlope > 1 {
            result.Trend = "CRITICAL_GROWTH"
        } else if trend.CPUSlope > 1 || trend.MemSlope > 0.5 {
            result.Trend = "GROWING"
        } else if trend.CPUSlope < -1 || trend.MemSlope < -0.5 {
            result.Trend = "DECLINING"
        } else {
            result.Trend = "STABLE"
        }
    }
    
    // === 3. RESTART PATTERN ANALYSIS ===
    if current.Restarts >= 3 {
        result.Issues = append(result.Issues, fmt.Sprintf("High restart count: %d", current.Restarts))
        score += 30
        result.Action = "INVESTIGATE_RESTARTS"
    }
    
    if current.Status != "Running" {
        result.Issues = append(result.Issues, fmt.Sprintf("Pod not running: %s", current.Status))
        result.Risk = "CRITICAL"
        result.Action = "RESTART_POD"
        score += 50
    }
    
    // === 4. FINALIZE RISK ASSESSMENT ===
    result.Score = math.Min(score, 100)
    
    if result.Score >= 80 {
        result.Risk = "CRITICAL"
        result.Confidence = 95
    } else if result.Score >= 60 {
        result.Risk = "HIGH"
        result.Confidence = 90
    } else if result.Score >= 40 {
        result.Risk = "MEDIUM"
        result.Confidence = 85
    } else if result.Score >= 20 {
        result.Risk = "LOW-MEDIUM"
        result.Confidence = 80
    } else {
        result.Risk = "LOW"
        result.Confidence = 100
    }
    
    return result
}

func (p *Predictor) calculateAdvancedTrend(history []collector.PodMetrics, current collector.PodMetrics) TrendAnalysis {
    if len(history) < 3 {
        return TrendAnalysis{CPUTrend: "UNKNOWN", MemTrend: "UNKNOWN"}
    }
    
    // Calculate time span in hours (measurements every 30 seconds)
    timeSpan := float64(len(history)) * 0.5 / 60.0 // Convert to hours
    if timeSpan == 0 {
        timeSpan = 0.5 // Minimum 30 minutes
    }
    
    // Calculate trends using linear regression for better accuracy
    cpuSlope := p.calculateSlope(history, current, "cpu")
    memSlope := p.calculateSlope(history, current, "memory")
    
    trend := TrendAnalysis{
        CPUSlope:      cpuSlope,
        MemSlope:      memSlope,
        IsMemoryLeak:  memSlope > 1,
        IsCPUGrowing:  cpuSlope > 2,
    }
    
    // Classify trends
    if cpuSlope > 5 {
        trend.CPUTrend = "RISING_FAST"
    } else if cpuSlope > 2 {
        trend.CPUTrend = "RISING"
    } else if cpuSlope < -5 {
        trend.CPUTrend = "FALLING_FAST"
    } else if cpuSlope < -2 {
        trend.CPUTrend = "FALLING"
    } else {
        trend.CPUTrend = "STABLE"
    }
    
    if memSlope > 3 {
        trend.MemTrend = "RISING_FAST"
    } else if memSlope > 1 {
        trend.MemTrend = "RISING"
    } else if memSlope < -3 {
        trend.MemTrend = "FALLING_FAST"
    } else if memSlope < -1 {
        trend.MemTrend = "FALLING"
    } else {
        trend.MemTrend = "STABLE"
    }
    
    return trend
}

func (p *Predictor) calculateSlope(history []collector.PodMetrics, current collector.PodMetrics, resourceType string) float64 {
    if len(history) < 2 {
        return 0
    }
    
    // Simple linear regression to find slope (change per hour)
    n := float64(len(history) + 1)
    timeSpan := n * 0.5 / 60.0 // hours
    
    var values []float64
    for _, h := range history {
        if resourceType == "cpu" {
            values = append(values, h.CPUPercent)
        } else {
            values = append(values, h.MemPercent)
        }
    }
    
    if resourceType == "cpu" {
        values = append(values, current.CPUPercent)
    } else {
        values = append(values, current.MemPercent)
    }
    
    // Calculate slope using first and last values (simplified)
    if len(values) >= 2 {
        first := values[0]
        last := values[len(values)-1]
        return (last - first) / timeSpan
    }
    
    return 0
}

func (p *Predictor) detectPerformanceDegradation(history []collector.PodMetrics, current collector.PodMetrics) bool {
    if len(history) < 5 {
        return false
    }
    
    // Check for gradual increase in resource usage without spikes
    cpuIncreases := 0
    memIncreases := 0
    
    for i := 1; i < len(history); i++ {
        if history[i].CPUPercent > history[i-1].CPUPercent {
            cpuIncreases++
        }
        if history[i].MemPercent > history[i-1].MemPercent {
            memIncreases++
        }
    }
    
    // Performance degradation if >70% of measurements show increases
    degradationThreshold := float64(len(history)) * 0.7
    return float64(cpuIncreases) > degradationThreshold || float64(memIncreases) > degradationThreshold
}

func (p *Predictor) PrintPredictions(predictions []PredictionResult) {
    if len(predictions) == 0 {
        fmt.Printf("🟢 All pods healthy - no issues predicted\n\n")
        return
    }
    
    fmt.Printf("🔮 === AI SMART PREDICTIONS & FORECASTS ===\n")
    for _, pred := range predictions {
        riskIcon := "🟡"
        if pred.Risk == "CRITICAL" {
            riskIcon = "🔴"
        } else if pred.Risk == "HIGH" {
            riskIcon = "🟠"
        } else if pred.Risk == "MEDIUM" {
            riskIcon = "🟡"
        }
        
        fmt.Printf("%s Pod: %s/%s - Risk: %s (Score: %.1f, %d%% confidence)\n", 
            riskIcon, pred.PodNamespace, pred.PodName, pred.Risk, pred.Score, pred.Confidence)
        
        if pred.TimeToFailure != "N/A" {
            fmt.Printf("  ⏰ PREDICTION: Failure in %s\n", pred.TimeToFailure)
        }
        
        if pred.MemoryLeakRate > 1 {
            fmt.Printf("  🩸 Memory leak: +%.1f%%/hour\n", pred.MemoryLeakRate)
        }
        
        if pred.CPUGrowthRate > 2 {
            fmt.Printf("  📈 CPU growth: +%.1f%%/hour\n", pred.CPUGrowthRate)
        }
        
        if pred.Trend != "STABLE" {
            fmt.Printf("  📊 Trend: %s\n", pred.Trend)
        }
        
        for _, issue := range pred.Issues {
            fmt.Printf("  ⚠️  %s\n", issue)
        }
        
        fmt.Printf("  💡 AI Action: %s\n\n", pred.Action)
    }
    fmt.Printf("=======================================\n\n")
}
