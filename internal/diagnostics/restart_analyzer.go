package diagnostics

import (
    "context"
    "fmt"
    "strings"
    "time"
    
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    corev1 "k8s.io/api/core/v1"
)

type RestartPattern struct {
    PodName       string
    Namespace     string
    RestartCount  int32
    Pattern       string
    Frequency     string
    Severity      string
    RootCause     string
    Actions       []string
}

func (d *DiagnosticsEngine) AnalyzeRestartPatterns(ctx context.Context, namespace string) ([]RestartPattern, error) {
    var patterns []RestartPattern
    
    listOptions := metav1.ListOptions{}
    if namespace == "" {
        namespace = metav1.NamespaceAll
    }
    
    pods, err := d.clientset.CoreV1().Pods(namespace).List(ctx, listOptions)
    if err != nil {
        return nil, fmt.Errorf("failed to list pods: %v", err)
    }
    
    for _, pod := range pods.Items {
        // Skip system pods
        if strings.Contains(pod.Namespace, "kube-") || 
           strings.Contains(pod.Namespace, "healer-") {
            continue
        }
        
        pattern := d.analyzeRestartPattern(pod)
        if pattern.RestartCount > 0 {
            patterns = append(patterns, pattern)
        }
    }
    
    return patterns, nil
}

func (d *DiagnosticsEngine) analyzeRestartPattern(pod corev1.Pod) RestartPattern {
    pattern := RestartPattern{
        PodName:      pod.Name,
        Namespace:    pod.Namespace,
        RestartCount: 0,
        Pattern:      "STABLE",
        Frequency:    "NONE",
        Severity:     "OK",
        RootCause:    "No restarts detected",
        Actions:      []string{},
    }
    
    // Count total restarts
    var totalRestarts int32
    for _, containerStatus := range pod.Status.ContainerStatuses {
        totalRestarts += containerStatus.RestartCount
    }
    
    pattern.RestartCount = totalRestarts
    
    if totalRestarts == 0 {
        return pattern
    }
    
    // Calculate restart frequency based on pod age
    podAge := time.Since(pod.CreationTimestamp.Time)
    
    // Determine restart frequency
    restartsPerHour := float64(totalRestarts) / podAge.Hours()
    
    if restartsPerHour > 2 {
        pattern.Frequency = "VERY_HIGH"
        pattern.Severity = "CRITICAL"
    } else if restartsPerHour > 1 {
        pattern.Frequency = "HIGH"
        pattern.Severity = "HIGH"
    } else if restartsPerHour > 0.5 {
        pattern.Frequency = "MEDIUM"
        pattern.Severity = "MEDIUM"
    } else {
        pattern.Frequency = "LOW"
        pattern.Severity = "LOW"
    }
    
    // Analyze restart patterns
    if totalRestarts >= 10 {
        pattern.Pattern = "CRASH_LOOP"
        pattern.RootCause = "Persistent application crashes"
        pattern.Actions = []string{"CHECK_LOGS", "ROLLBACK_DEPLOYMENT", "CHECK_RESOURCES"}
    } else if totalRestarts >= 5 && podAge < 1*time.Hour {
        pattern.Pattern = "RAPID_RESTART"
        pattern.RootCause = "Fast restart cycle - likely config issue"
        pattern.Actions = []string{"CHECK_CONFIG", "CHECK_LOGS", "RESTART_POD"}
    } else if totalRestarts >= 3 && podAge < 30*time.Minute {
        pattern.Pattern = "STARTUP_FAILURE"
        pattern.RootCause = "Application failing to start properly"
        pattern.Actions = []string{"CHECK_STARTUP_PROBE", "CHECK_DEPENDENCIES", "CHECK_LOGS"}
    } else if restartsPerHour > 0.1 {
        pattern.Pattern = "PERIODIC_RESTART"
        pattern.RootCause = "Regular restart pattern - possible memory leak"
        pattern.Actions = []string{"CHECK_MEMORY_LEAK", "MONITOR_RESOURCES", "CHECK_LOGS"}
    }
    
    // Analyze container exit reasons
    for _, containerStatus := range pod.Status.ContainerStatuses {
        if containerStatus.LastTerminationState.Terminated != nil {
            exitCode := containerStatus.LastTerminationState.Terminated.ExitCode
            reason := containerStatus.LastTerminationState.Terminated.Reason
            
            if exitCode == 137 { // SIGKILL
                pattern.RootCause = "Container killed by OOM or system"
                pattern.Actions = append(pattern.Actions, "INCREASE_MEMORY", "CHECK_OOM")
            } else if exitCode == 143 { // SIGTERM
                pattern.RootCause = "Container gracefully terminated"
                pattern.Actions = append(pattern.Actions, "CHECK_SHUTDOWN_HOOKS")
            } else if exitCode == 1 {
                pattern.RootCause = "Application error exit"
                pattern.Actions = append(pattern.Actions, "CHECK_APPLICATION_LOGS", "DEBUG_APPLICATION")
            }
            
            if reason == "OOMKilled" {
                pattern.RootCause = "Out of Memory killed"
                pattern.Severity = "CRITICAL"
                pattern.Actions = []string{"INCREASE_MEMORY_LIMITS", "CHECK_MEMORY_LEAK", "OPTIMIZE_MEMORY"}
            }
        }
    }
    
    return pattern
}

func (d *DiagnosticsEngine) PrintRestartAnalysis(patterns []RestartPattern) {
    if len(patterns) == 0 {
        fmt.Printf("🟢 No restart issues detected\n\n")
        return
    }
    
    fmt.Printf("🔄 === RESTART PATTERN ANALYSIS ===\n")
    for _, pattern := range patterns {
        severityIcon := "🟡"
        if pattern.Severity == "CRITICAL" {
            severityIcon = "🔴"
        } else if pattern.Severity == "HIGH" {
            severityIcon = "🟠"
        }
        
        fmt.Printf("%s Pod: %s/%s\n", severityIcon, pattern.Namespace, pattern.PodName)
        fmt.Printf("  📊 Restarts: %d | Pattern: %s | Frequency: %s\n", 
            pattern.RestartCount, pattern.Pattern, pattern.Frequency)
        fmt.Printf("  🔍 Root Cause: %s\n", pattern.RootCause)
        if len(pattern.Actions) > 0 {
            fmt.Printf("  💡 Actions: %v\n", pattern.Actions)
        }
        fmt.Printf("\n")
    }
    fmt.Printf("===================================\n\n")
}
