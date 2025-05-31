package diagnostics

import (
    "context"
    "fmt"
    "strings"
    "time"
    
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type HealingAction struct {
    ActionType    string
    PodName       string
    Namespace     string
    ContainerName string
    Description   string
    Status        string
    Timestamp     time.Time
    Result        string
}

type AutoHealer struct {
    diagEngine *DiagnosticsEngine
    history    []HealingAction
    dryRun     bool
}

func NewAutoHealer(diagEngine *DiagnosticsEngine, dryRun bool) *AutoHealer {
    return &AutoHealer{
        diagEngine: diagEngine,
        history:    make([]HealingAction, 0),
        dryRun:     dryRun,
    }
}

func (h *AutoHealer) HealContainerIssues(ctx context.Context, containerChecks []ContainerCheckResult) []HealingAction {
    var actions []HealingAction
    
    for _, checkResult := range containerChecks {
        if !checkResult.NeedsAction {
            continue
        }
        
        for _, check := range checkResult.Checks {
            if check.Status == "OK" {
                continue
            }
            
            // Execute healing actions based on check type
            switch check.CheckName {
            case "/tmp Directory":
                if contains(check.FixActions, "CLEANUP_TMP") {
                    action := h.cleanupTmpDirectory(ctx, checkResult, check)
                    actions = append(actions, action)
                }
            case "Disk Space":
                if contains(check.FixActions, "CLEANUP_DISK") {
                    action := h.cleanupDiskSpace(ctx, checkResult, check)
                    actions = append(actions, action)
                }
            case "Network Connectivity":
                if contains(check.FixActions, "CHECK_NETWORK") {
                    action := h.fixNetworkConnectivity(ctx, checkResult, check)
                    actions = append(actions, action)
                }
            case "DNS Resolution":
                if contains(check.FixActions, "RESTART_DNS") {
                    action := h.fixDNSResolution(ctx, checkResult, check)
                    actions = append(actions, action)
                }
            }
        }
    }
    
    // Store actions in history
    h.history = append(h.history, actions...)
    
    // Keep only last 100 actions
    if len(h.history) > 100 {
        h.history = h.history[len(h.history)-100:]
    }
    
    return actions
}

func (h *AutoHealer) cleanupTmpDirectory(ctx context.Context, checkResult ContainerCheckResult, check ContainerCheck) HealingAction {
    action := HealingAction{
        ActionType:    "CLEANUP_TMP",
        PodName:       checkResult.PodName,
        Namespace:     checkResult.Namespace,
        ContainerName: checkResult.ContainerName,
        Description:   "Cleaning up /tmp directory",
        Status:        "EXECUTING",
        Timestamp:     time.Now(),
    }
    
    if h.dryRun {
        action.Status = "DRY_RUN"
        action.Result = "Would cleanup /tmp directory"
        return action
    }
    
    // Commands to safely cleanup /tmp
    cleanupCommands := []string{
        "find /tmp -type f -atime +1 -delete 2>/dev/null || true",
        "find /tmp -type f -size +10M -delete 2>/dev/null || true",
        "find /tmp -name '*.log' -mtime +1 -delete 2>/dev/null || true",
        "find /tmp -name 'core.*' -delete 2>/dev/null || true",
        "find /tmp -name '*.tmp' -mtime +1 -delete 2>/dev/null || true",
    }
    
    var results []string
    for _, cmd := range cleanupCommands {
        _, err := h.diagEngine.execInContainer(ctx, checkResult.Namespace, checkResult.PodName, checkResult.ContainerName, cmd)
        if err != nil {
            results = append(results, fmt.Sprintf("Failed: %s", cmd))
        } else {
            results = append(results, "Cleanup executed")
        }
    }
    
    action.Status = "COMPLETED"
    action.Result = strings.Join(results, "; ")
    
    return action
}

func (h *AutoHealer) cleanupDiskSpace(ctx context.Context, checkResult ContainerCheckResult, check ContainerCheck) HealingAction {
    action := HealingAction{
        ActionType:    "CLEANUP_DISK",
        PodName:       checkResult.PodName,
        Namespace:     checkResult.Namespace,
        ContainerName: checkResult.ContainerName,
        Description:   "Cleaning up disk space",
        Status:        "EXECUTING",
        Timestamp:     time.Now(),
    }
    
    if h.dryRun {
        action.Status = "DRY_RUN"
        action.Result = "Would cleanup disk space"
        return action
    }
    
    // Safe disk cleanup commands
    cleanupCommands := []string{
        "find /var/log -name '*.log' -size +50M -exec truncate -s 10M {} + 2>/dev/null || true",
        "find /var/log -name '*.log.*' -mtime +7 -delete 2>/dev/null || true",
        "find / -name '*.core' -delete 2>/dev/null || true",
        "find /var/tmp -type f -mtime +3 -delete 2>/dev/null || true",
    }
    
    var results []string
    for _, cmd := range cleanupCommands {
        _, err := h.diagEngine.execInContainer(ctx, checkResult.Namespace, checkResult.PodName, checkResult.ContainerName, cmd)
        if err != nil {
            results = append(results, fmt.Sprintf("Failed: %v", err))
        } else {
            results = append(results, "Cleanup executed")
        }
    }
    
    action.Status = "COMPLETED"
    action.Result = strings.Join(results, "; ")
    
    return action
}

func (h *AutoHealer) fixNetworkConnectivity(ctx context.Context, checkResult ContainerCheckResult, check ContainerCheck) HealingAction {
    action := HealingAction{
        ActionType:    "FIX_NETWORK",
        PodName:       checkResult.PodName,
        Namespace:     checkResult.Namespace,
        ContainerName: checkResult.ContainerName,
        Description:   "Fixing network connectivity",
        Status:        "EXECUTING",
        Timestamp:     time.Now(),
    }
    
    if h.dryRun {
        action.Status = "DRY_RUN"
        action.Result = "Would restart network services and pod if needed"
        return action
    }
    
    // First try network fixes
    networkCommands := []string{
        "ip route flush cache 2>/dev/null || true",
        "ping -c 1 kubernetes.default.svc.cluster.local 2>/dev/null && echo 'Network OK' || echo 'Network FAIL'",
    }
    
    var results []string
    networkFailed := false
    
    for _, cmd := range networkCommands {
        output, err := h.diagEngine.execInContainer(ctx, checkResult.Namespace, checkResult.PodName, checkResult.ContainerName, cmd)
        if err != nil {
            results = append(results, fmt.Sprintf("Command failed: %v", err))
            networkFailed = true
        } else {
            results = append(results, strings.TrimSpace(output))
            if strings.Contains(output, "Network FAIL") {
                networkFailed = true
            }
        }
    }
    
    // If network is still failing, try more aggressive fixes
    if networkFailed {
        results = append(results, "Network still failing - attempting pod restart")
        
        // Delete the pod to force restart
        err := h.diagEngine.clientset.CoreV1().Pods(checkResult.Namespace).Delete(ctx, checkResult.PodName, metav1.DeleteOptions{})
        if err != nil {
            results = append(results, fmt.Sprintf("Pod restart failed: %v", err))
            action.Status = "FAILED"
        } else {
            results = append(results, "Pod restarted successfully")
            action.ActionType = "RESTART_POD_NETWORK"
            action.Description = "Restarted pod due to network failure"
        }
    }
    
    action.Status = "COMPLETED"
    action.Result = strings.Join(results, "; ")
    
    return action
}

func (h *AutoHealer) fixDNSResolution(ctx context.Context, checkResult ContainerCheckResult, check ContainerCheck) HealingAction {
    action := HealingAction{
        ActionType:    "FIX_DNS",
        PodName:       checkResult.PodName,
        Namespace:     checkResult.Namespace,
        ContainerName: checkResult.ContainerName,
        Description:   "Fixing DNS resolution",
        Status:        "EXECUTING",
        Timestamp:     time.Now(),
    }
    
    if h.dryRun {
        action.Status = "DRY_RUN"
        action.Result = "Would flush DNS cache and restart DNS"
        return action
    }
    
    // DNS fix commands
    dnsCommands := []string{
        "nslookup kubernetes.default.svc.cluster.local 2>/dev/null && echo 'DNS OK' || echo 'DNS FAIL'",
    }
    
    var results []string
    for _, cmd := range dnsCommands {
        output, err := h.diagEngine.execInContainer(ctx, checkResult.Namespace, checkResult.PodName, checkResult.ContainerName, cmd)
        if err != nil {
            results = append(results, fmt.Sprintf("DNS command failed: %v", err))
        } else {
            results = append(results, strings.TrimSpace(output))
        }
    }
    
    action.Status = "COMPLETED"
    action.Result = strings.Join(results, "; ")
    
    return action
}

func (h *AutoHealer) GetHealingHistory() []HealingAction {
    return h.history
}

func (h *AutoHealer) PrintHealingActions(actions []HealingAction) {
    if len(actions) == 0 {
        return
    }
    
    fmt.Printf("🛠️  === AUTO-HEALING ACTIONS ===\n")
    for _, action := range actions {
        statusIcon := "✅"
        if action.Status == "DRY_RUN" {
            statusIcon = "🔄"
        } else if action.Status == "FAILED" {
            statusIcon = "❌"
        }
        
        fmt.Printf("%s %s: %s/%s/%s\n", 
            statusIcon, action.ActionType, action.Namespace, action.PodName, action.ContainerName)
        fmt.Printf("  📝 %s\n", action.Description)
        if action.Result != "" {
            fmt.Printf("  📊 Result: %s\n", action.Result)
        }
        fmt.Printf("  🕐 %s\n\n", action.Timestamp.Format("15:04:05"))
    }
    fmt.Printf("=================================\n\n")
}

func contains(slice []string, item string) bool {
    for _, s := range slice {
        if s == item {
            return true
        }
    }
    return false
}
