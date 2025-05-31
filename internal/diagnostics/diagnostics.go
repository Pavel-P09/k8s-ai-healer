package diagnostics

import (
    "context"
    "fmt"
    "strconv"
    "strings"
    "time"

    "k8s.io/client-go/kubernetes"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/client-go/kubernetes/scheme"
    "k8s.io/client-go/tools/remotecommand"
    "k8s.io/client-go/rest"
    corev1 "k8s.io/api/core/v1"
)

type DiagnosticsEngine struct {
    clientset *kubernetes.Clientset
    config    *rest.Config
    history   map[string][]ContainerStats
}

type ContainerStats struct {
    Timestamp    time.Time
    CPUIOwait    float64
    DiskReadMB   float64
    DiskWriteMB  float64
    NetworkRxMB  float64
    NetworkTxMB  float64
    ProcessCount int
    IsStuck      bool
}

type DiagnosticResult struct {
    PodName      string
    Namespace    string
    ContainerName string
    IsStuck      bool
    StuckReason  string
    LastActivity time.Time
    Severity     string
    Actions      []string
}

func New(clientset *kubernetes.Clientset, config *rest.Config) *DiagnosticsEngine {
    return &DiagnosticsEngine{
        clientset: clientset,
        config:    config,
        history:   make(map[string][]ContainerStats),
    }
}

func (d *DiagnosticsEngine) DiagnoseStuckContainers(ctx context.Context, namespace string) ([]DiagnosticResult, error) {
    var results []DiagnosticResult
    
    listOptions := metav1.ListOptions{}
    if namespace == "" {
        namespace = metav1.NamespaceAll
    }
    
    pods, err := d.clientset.CoreV1().Pods(namespace).List(ctx, listOptions)
    if err != nil {
        return nil, fmt.Errorf("failed to list pods: %v", err)
    }
    
    for _, pod := range pods.Items {
        if pod.Status.Phase != "Running" {
            continue
        }
        
        // Skip system pods
        if strings.Contains(pod.Namespace, "kube-") || 
           strings.Contains(pod.Namespace, "healer-") {
            continue
        }
        
        for _, container := range pod.Spec.Containers {
            result := d.analyzeContainer(ctx, pod.Namespace, pod.Name, container.Name)
            if result.IsStuck {
                results = append(results, result)
            }
        }
    }
    
    return results, nil
}

func (d *DiagnosticsEngine) analyzeContainer(ctx context.Context, namespace, podName, containerName string) DiagnosticResult {
    result := DiagnosticResult{
        PodName:       podName,
        Namespace:     namespace,
        ContainerName: containerName,
        IsStuck:       false,
        Severity:      "OK",
        Actions:       []string{},
    }
    
    // Get current stats
    stats, err := d.getContainerStats(ctx, namespace, podName, containerName)
    if err != nil {
        result.StuckReason = fmt.Sprintf("Failed to get stats: %v", err)
        return result
    }
    
    // Store in history
    key := fmt.Sprintf("%s/%s/%s", namespace, podName, containerName)
    if d.history[key] == nil {
        d.history[key] = make([]ContainerStats, 0)
    }
    d.history[key] = append(d.history[key], stats)
    
    // Keep only last 10 measurements
    if len(d.history[key]) > 10 {
        d.history[key] = d.history[key][1:]
    }
    
    // Analyze for stuck patterns
    if len(d.history[key]) >= 3 {
        isStuck, reason := d.detectStuckContainer(d.history[key])
        if isStuck {
            result.IsStuck = true
            result.StuckReason = reason
            result.Severity = "CRITICAL"
            result.Actions = d.generateActions(reason)
        }
    }
    
    return result
}

func (d *DiagnosticsEngine) getContainerStats(ctx context.Context, namespace, podName, containerName string) (ContainerStats, error) {
    stats := ContainerStats{
        Timestamp: time.Now(),
    }
    
    // Simple commands that work in most containers
    commands := map[string]string{
        "proc_count": "ps aux 2>/dev/null | wc -l || echo 0",
        "uptime":     "uptime 2>/dev/null || echo '0.0 0.0 0.0'",
        "disk_usage": "df / 2>/dev/null | tail -1 | awk '{print $5}' || echo '0%'",
    }
    
    for metric, cmd := range commands {
        output, err := d.execInContainer(ctx, namespace, podName, containerName, cmd)
        if err != nil {
            // If exec fails, container might be stuck
            if metric == "proc_count" {
                stats.IsStuck = true
            }
            continue
        }
        
        switch metric {
        case "proc_count":
            if value, err := strconv.Atoi(strings.TrimSpace(output)); err == nil {
                stats.ProcessCount = value
            }
        case "uptime":
            // Parse load average from uptime
            parts := strings.Fields(output)
            if len(parts) >= 3 {
                if load, err := strconv.ParseFloat(parts[len(parts)-3], 64); err == nil {
                    stats.CPUIOwait = load * 100 // Simplified load to percentage
                }
            }
        }
    }
    
    return stats, nil
}

func (d *DiagnosticsEngine) execInContainer(ctx context.Context, namespace, podName, containerName, command string) (string, error) {
    req := d.clientset.CoreV1().RESTClient().Post().
        Resource("pods").
        Name(podName).
        Namespace(namespace).
        SubResource("exec")
    
    // Correct way to set exec parameters
    execOptions := &corev1.PodExecOptions{
        Container: containerName,
        Command:   []string{"/bin/sh", "-c", command},
        Stdout:    true,
        Stderr:    true,
    }
    
    req.VersionedParams(execOptions, scheme.ParameterCodec)
    
    exec, err := remotecommand.NewSPDYExecutor(d.config, "POST", req.URL())
    if err != nil {
        return "", err
    }
    
    var stdout, stderr strings.Builder
    err = exec.Stream(remotecommand.StreamOptions{
        Stdout: &stdout,
        Stderr: &stderr,
    })
    
    if err != nil {
        return "", err
    }
    
    return stdout.String(), nil
}

func (d *DiagnosticsEngine) detectStuckContainer(history []ContainerStats) (bool, string) {
    if len(history) < 3 {
        return false, ""
    }
    
    recent := history[len(history)-3:]
    
    // Check if any stats show container is stuck
    for _, stat := range recent {
        if stat.IsStuck {
            return true, "Container exec commands failing - container may be unresponsive"
        }
    }
    
    // Check for consistently high load
    highLoad := true
    for _, stat := range recent {
        if stat.CPUIOwait < 80 {
            highLoad = false
            break
        }
    }
    if highLoad {
        return true, "Consistently high system load - container may be stuck"
    }
    
    // Check for zero or very low process count
    lowProcesses := true
    for _, stat := range recent {
        if stat.ProcessCount > 3 {
            lowProcesses = false
            break
        }
    }
    if lowProcesses {
        return true, "Very low process count - container may be in minimal state"
    }
    
    // Check for decreasing process count over time
    if len(recent) >= 2 {
        first := recent[0].ProcessCount
        last := recent[len(recent)-1].ProcessCount
        if first > 0 && last < first-3 {
            return true, "Process count decreasing rapidly - application may be failing"
        }
    }
    
    return false, ""
}

func (d *DiagnosticsEngine) generateActions(reason string) []string {
    actions := []string{}
    
    if strings.Contains(reason, "unresponsive") || strings.Contains(reason, "failing") {
        actions = append(actions, "RESTART_POD", "CHECK_LOGS")
    }
    if strings.Contains(reason, "high load") || strings.Contains(reason, "stuck") {
        actions = append(actions, "RESTART_POD", "CHECK_RESOURCES")
    }
    if strings.Contains(reason, "process count") {
        actions = append(actions, "RESTART_POD", "INVESTIGATE_APP")
    }
    if strings.Contains(reason, "minimal state") {
        actions = append(actions, "CHECK_HEALTH", "MONITOR_CLOSELY")
    }
    
    return actions
}

func (d *DiagnosticsEngine) PrintDiagnostics(results []DiagnosticResult) {
    if len(results) == 0 {
        fmt.Printf("🟢 No stuck containers detected\n\n")
        return
    }
    
    fmt.Printf("🔍 === STUCK CONTAINER DIAGNOSTICS ===\n")
    for _, result := range results {
        severityIcon := "🟡"
        if result.Severity == "CRITICAL" {
            severityIcon = "🔴"
        }
        
        fmt.Printf("%s Container: %s/%s/%s - STUCK DETECTED\n", 
            severityIcon, result.Namespace, result.PodName, result.ContainerName)
        fmt.Printf("  ⚠️  Reason: %s\n", result.StuckReason)
        fmt.Printf("  💡 Recommended Actions: %v\n\n", result.Actions)
    }
    fmt.Printf("======================================\n\n")
}
