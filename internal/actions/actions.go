package actions

import (
    "context"
    "fmt"
    "log"
    "time"

    "k8s-healer/internal/predictor"

    "k8s.io/client-go/kubernetes"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ActionEngine struct {
    clientset    *kubernetes.Clientset
    dryRun       bool
    actionCounts map[string]int
}

func New(clientset *kubernetes.Clientset, dryRun bool) *ActionEngine {
    return &ActionEngine{
        clientset:    clientset,
        dryRun:       dryRun,
        actionCounts: make(map[string]int),
    }
}

func (a *ActionEngine) ExecuteActions(predictions []predictor.PredictionResult) {
    if len(predictions) == 0 {
        return
    }

    fmt.Printf("🤖 === EXECUTING HEALING ACTIONS ===\n")
    
    for _, pred := range predictions {
        key := fmt.Sprintf("%s/%s", pred.PodNamespace, pred.PodName)
        
        if a.actionCounts[key] >= 3 {
            fmt.Printf("⚠️  Skipping %s - max actions reached (3)\n", key)
            continue
        }
        
        switch pred.Action {
        case "SCALE_UP_URGENT":
            a.scaleUpDeployment(pred)
        case "SCALE_UP":
            a.scaleUpDeployment(pred)
        case "RESTART_POD_URGENT":
            a.restartPod(pred)
        case "RESTART_POD":
            a.restartPod(pred)
        case "INVESTIGATE_RESTARTS":
            a.investigatePod(pred)
        case "MONITOR_CLOSELY":
            a.monitorPod(pred)
        default:
            fmt.Printf("📊 Monitoring: %s/%s\n", pred.PodNamespace, pred.PodName)
        }
        
        a.actionCounts[key]++
    }
    
    fmt.Printf("=====================================\n\n")
}

func (a *ActionEngine) scaleUpDeployment(pred predictor.PredictionResult) {
    if a.dryRun {
        fmt.Printf("🚀 [DRY RUN] Would scale UP deployment for pod: %s/%s (CPU overload)\n", 
            pred.PodNamespace, pred.PodName)
        return
    }
    
    ctx := context.TODO()
    deployments, err := a.clientset.AppsV1().Deployments(pred.PodNamespace).List(ctx, metav1.ListOptions{})
    if err != nil {
        fmt.Printf("❌ Failed to list deployments: %v\n", err)
        return
    }
    
    for _, dep := range deployments.Items {
        if len(pred.PodName) > len(dep.Name) && pred.PodName[:len(dep.Name)] == dep.Name {
            currentReplicas := *dep.Spec.Replicas
            newReplicas := currentReplicas + 1
            dep.Spec.Replicas = &newReplicas
            
            _, err := a.clientset.AppsV1().Deployments(pred.PodNamespace).Update(ctx, &dep, metav1.UpdateOptions{})
            if err != nil {
                fmt.Printf("❌ Failed to scale deployment: %v\n", err)
                return
            }
            
            fmt.Printf("🚀 AUTO-SCALED deployment %s from %d to %d replicas (CPU overload detected)\n", 
                dep.Name, currentReplicas, newReplicas)
            a.logAction("AUTO_SCALE_UP", pred)
            break
        }
    }
}

func (a *ActionEngine) restartPod(pred predictor.PredictionResult) {
    ctx := context.TODO()
    
    if a.dryRun {
        fmt.Printf("🔄 [DRY RUN] Would restart pod: %s/%s (Memory/Status issue)\n", 
            pred.PodNamespace, pred.PodName)
        return
    }
    
    err := a.clientset.CoreV1().Pods(pred.PodNamespace).Delete(ctx, pred.PodName, metav1.DeleteOptions{})
    if err != nil {
        log.Printf("❌ Failed to restart pod %s/%s: %v", pred.PodNamespace, pred.PodName, err)
        return
    }
    
    fmt.Printf("🔄 AUTO-RESTARTED pod: %s/%s (Memory/Status issue detected)\n", 
        pred.PodNamespace, pred.PodName)
    a.logAction("AUTO_RESTART", pred)
}

func (a *ActionEngine) investigatePod(pred predictor.PredictionResult) {
    fmt.Printf("🔍 INVESTIGATING pod: %s/%s (Restart pattern detected)\n", 
        pred.PodNamespace, pred.PodName)
    
    ctx := context.TODO()
    events, err := a.clientset.CoreV1().Events(pred.PodNamespace).List(ctx, metav1.ListOptions{
        FieldSelector: fmt.Sprintf("involvedObject.name=%s", pred.PodName),
    })
    
    if err == nil && len(events.Items) > 0 {
        fmt.Printf("  📋 Recent events:\n")
        for i, event := range events.Items {
            if i >= 3 {
                break
            }
            fmt.Printf("    - %s: %s\n", event.Reason, event.Message)
        }
    }
    
    a.logAction("INVESTIGATE", pred)
}

func (a *ActionEngine) monitorPod(pred predictor.PredictionResult) {
    fmt.Printf("👀 MONITORING CLOSELY: %s/%s (Resource usage trending up)\n", 
        pred.PodNamespace, pred.PodName)
    a.logAction("MONITOR", pred)
}

func (a *ActionEngine) logAction(action string, pred predictor.PredictionResult) {
    timestamp := time.Now().Format("15:04:05")
    fmt.Printf("  📝 [%s] AI Action: %s for %s/%s (Risk: %s)\n", 
        timestamp, action, pred.PodNamespace, pred.PodName, pred.Risk)
}

func (a *ActionEngine) GetActionCounts() map[string]int {
    return a.actionCounts
}
