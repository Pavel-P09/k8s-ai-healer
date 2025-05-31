package diagnostics
import (
    "context"
    "fmt"
    "strconv"
    "strings"
    
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)
type ContainerCheck struct {
    CheckName   string
    Status      string
    Details     string
    Severity    string
    FixActions  []string
}

type ContainerCheckResult struct {
    PodName       string
    Namespace     string
    ContainerName string
    Checks        []ContainerCheck
    OverallStatus string
    NeedsAction   bool
}

func (d *DiagnosticsEngine) RunContainerChecks(ctx context.Context, namespace string) ([]ContainerCheckResult, error) {
    var results []ContainerCheckResult
    
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
            result := d.checkContainer(ctx, pod.Namespace, pod.Name, container.Name)
            if result.NeedsAction {
                results = append(results, result)
            }
        }
    }
    
    return results, nil
}

func (d *DiagnosticsEngine) checkContainer(ctx context.Context, namespace, podName, containerName string) ContainerCheckResult {
    result := ContainerCheckResult{
        PodName:       podName,
        Namespace:     namespace,
        ContainerName: containerName,
        Checks:        []ContainerCheck{},
        OverallStatus: "OK",
        NeedsAction:   false,
    }
    
    // DNS Check
    dnsCheck := d.checkDNS(ctx, namespace, podName, containerName)
    result.Checks = append(result.Checks, dnsCheck)
    
    // Disk Space Check
    diskCheck := d.checkDiskSpace(ctx, namespace, podName, containerName)
    result.Checks = append(result.Checks, diskCheck)
    
    // /tmp Directory Check
    tmpCheck := d.checkTmpDirectory(ctx, namespace, podName, containerName)
    result.Checks = append(result.Checks, tmpCheck)
    
    // Network Connectivity Check
    networkCheck := d.checkNetworkConnectivity(ctx, namespace, podName, containerName)
    result.Checks = append(result.Checks, networkCheck)
    
    // Determine overall status
    for _, check := range result.Checks {
        if check.Status == "CRITICAL" {
            result.OverallStatus = "CRITICAL"
            result.NeedsAction = true
        } else if check.Status == "WARNING" && result.OverallStatus != "CRITICAL" {
            result.OverallStatus = "WARNING"
            result.NeedsAction = true
        }
    }
    
    return result
}

func (d *DiagnosticsEngine) checkDNS(ctx context.Context, namespace, podName, containerName string) ContainerCheck {
    check := ContainerCheck{
        CheckName:  "DNS Resolution",
        Status:     "OK",
        Details:    "DNS working normally",
        Severity:   "LOW",
        FixActions: []string{},
    }
    
    // Test DNS resolution for Kubernetes internal services
    dnsCommands := []string{
        "nslookup kubernetes.default.svc.cluster.local 2>/dev/null | grep 'Name:' || echo 'DNS_FAIL'",
        "nslookup google.com 2>/dev/null | grep 'Name:' || echo 'EXTERNAL_DNS_FAIL'",
    }
    
    for i, cmd := range dnsCommands {
        output, err := d.execInContainer(ctx, namespace, podName, containerName, cmd)
        if err != nil || strings.Contains(output, "DNS_FAIL") {
            if i == 0 {
                check.Status = "CRITICAL"
                check.Details = "Internal Kubernetes DNS resolution failed"
                check.Severity = "HIGH"
                check.FixActions = []string{"RESTART_POD", "CHECK_DNS_CONFIG", "RESTART_DNS"}
            } else {
                check.Status = "WARNING"
                check.Details = "External DNS resolution failed"
                check.Severity = "MEDIUM"
                check.FixActions = []string{"CHECK_NETWORK", "CHECK_DNS_SERVERS"}
            }
            break
        }
    }
    
    return check
}

func (d *DiagnosticsEngine) checkDiskSpace(ctx context.Context, namespace, podName, containerName string) ContainerCheck {
    check := ContainerCheck{
        CheckName:  "Disk Space",
        Status:     "OK",
        Details:    "Disk space normal",
        Severity:   "LOW",
        FixActions: []string{},
    }
    
    // Check root filesystem usage
    cmd := "df / 2>/dev/null | tail -1 | awk '{print $5}' | sed 's/%//'"
    output, err := d.execInContainer(ctx, namespace, podName, containerName, cmd)
    if err != nil {
        check.Status = "WARNING"
        check.Details = "Could not check disk space"
        return check
    }
    
    usage, err := strconv.Atoi(strings.TrimSpace(output))
    if err != nil {
        check.Status = "WARNING"
        check.Details = "Invalid disk usage data"
        return check
    }
    
    if usage > 90 {
        check.Status = "CRITICAL"
        check.Details = fmt.Sprintf("Root filesystem %d%% full", usage)
        check.Severity = "HIGH"
        check.FixActions = []string{"CLEANUP_DISK", "RESTART_POD", "SCALE_STORAGE"}
    } else if usage > 80 {
        check.Status = "WARNING"
        check.Details = fmt.Sprintf("Root filesystem %d%% full", usage)
        check.Severity = "MEDIUM"
        check.FixActions = []string{"CLEANUP_DISK", "MONITOR_DISK"}
    } else {
        check.Details = fmt.Sprintf("Root filesystem %d%% used", usage)
    }
    
    return check
}

func (d *DiagnosticsEngine) checkTmpDirectory(ctx context.Context, namespace, podName, containerName string) ContainerCheck {
    check := ContainerCheck{
        CheckName:  "/tmp Directory",
        Status:     "OK",
        Details:    "/tmp directory normal",
        Severity:   "LOW",
        FixActions: []string{},
    }
    
    // Check /tmp directory usage
    cmd := "df /tmp 2>/dev/null | tail -1 | awk '{print $5}' | sed 's/%//' || echo '0'"
    output, err := d.execInContainer(ctx, namespace, podName, containerName, cmd)
    if err != nil {
        return check // /tmp might not exist or not be mounted separately
    }
    
    usage, err := strconv.Atoi(strings.TrimSpace(output))
    if err != nil {
        return check
    }
    
    if usage > 95 {
        check.Status = "CRITICAL"
        check.Details = fmt.Sprintf("/tmp directory %d%% full", usage)
        check.Severity = "HIGH"
        check.FixActions = []string{"CLEANUP_TMP", "RESTART_POD"}
    } else if usage > 85 {
        check.Status = "WARNING"
        check.Details = fmt.Sprintf("/tmp directory %d%% full", usage)
        check.Severity = "MEDIUM"
        check.FixActions = []string{"CLEANUP_TMP"}
    }
    
    // Check for large files in /tmp
    cmd2 := "find /tmp -type f -size +10M 2>/dev/null | wc -l"
    output2, err := d.execInContainer(ctx, namespace, podName, containerName, cmd2)
    if err == nil {
        if largeFiles, err := strconv.Atoi(strings.TrimSpace(output2)); err == nil && largeFiles > 0 {
            check.Details += fmt.Sprintf(", %d large files found", largeFiles)
            if check.Status == "OK" {
                check.Status = "WARNING"
                check.FixActions = []string{"CLEANUP_TMP"}
            }
        }
    }
    
    return check
}

func (d *DiagnosticsEngine) checkNetworkConnectivity(ctx context.Context, namespace, podName, containerName string) ContainerCheck {
    check := ContainerCheck{
        CheckName:  "Network Connectivity",
        Status:     "OK",
        Details:    "Network connectivity normal",
        Severity:   "LOW",
        FixActions: []string{},
    }
    
    // Test internal cluster connectivity
    cmd := "wget -q --timeout=5 --tries=1 -O /dev/null http://kubernetes.default.svc.cluster.local:443 2>/dev/null && echo 'OK' || echo 'FAIL'"
    output, err := d.execInContainer(ctx, namespace, podName, containerName, cmd)
    if err != nil || strings.Contains(output, "FAIL") {
        check.Status = "WARNING"
        check.Details = "Internal cluster connectivity issues"
        check.Severity = "MEDIUM"
        check.FixActions = []string{"CHECK_NETWORK", "RESTART_POD"}
    }
    
    // Test external connectivity
    cmd2 := "wget -q --timeout=5 --tries=1 -O /dev/null http://google.com 2>/dev/null && echo 'OK' || echo 'FAIL'"
    output2, err := d.execInContainer(ctx, namespace, podName, containerName, cmd2)
    if err != nil || strings.Contains(output2, "FAIL") {
        if check.Status == "OK" {
            check.Status = "WARNING"
            check.Details = "External connectivity issues"
            check.Severity = "LOW"
            check.FixActions = []string{"CHECK_EXTERNAL_NETWORK"}
        }
    }
    
    return check
}

func (d *DiagnosticsEngine) PrintContainerChecks(results []ContainerCheckResult) {
    if len(results) == 0 {
        fmt.Printf("🟢 All container checks passed\n\n")
        return
    }
    
    fmt.Printf("🔧 === CONTAINER HEALTH CHECKS ===\n")
    for _, result := range results {
        statusIcon := "🟡"
        if result.OverallStatus == "CRITICAL" {
            statusIcon = "🔴"
        } else if result.OverallStatus == "WARNING" {
            statusIcon = "🟠"
        }
        
        fmt.Printf("%s Container: %s/%s/%s - %s\n", 
            statusIcon, result.Namespace, result.PodName, result.ContainerName, result.OverallStatus)
        
        for _, check := range result.Checks {
            if check.Status != "OK" {
                fmt.Printf("  ❌ %s: %s\n", check.CheckName, check.Details)
                if len(check.FixActions) > 0 {
                    fmt.Printf("     💡 Actions: %v\n", check.FixActions)
                }
            }
        }
        fmt.Printf("\n")
    }
    fmt.Printf("==================================\n\n")
}
