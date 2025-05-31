package main

import (
    "context"
    "fmt"
    "io"
    "log"
    "path/filepath"
    "time"

    "k8s-healer/internal/collector"
    "k8s-healer/internal/predictor"
    "k8s-healer/internal/actions"
    "k8s-healer/internal/diagnostics"
    "k8s-healer/internal/api"

    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/rest"
    "k8s.io/client-go/tools/clientcmd"
    "k8s.io/client-go/util/homedir"
    metricsclient "k8s.io/metrics/pkg/client/clientset/versioned"
)

func main() {
    log.SetOutput(io.Discard)
    fmt.Println("🤖 K8s AI Healer v4.0 - COMPLETE SYSTEM WITH API")
    
    clientset, metricsClient, config, err := createClients()
    if err != nil {
        fmt.Printf("Failed to connect: %v\n", err)
        return
    }
    
    fmt.Println("✅ Connected to cluster")
    
    col := collector.New(clientset, metricsClient)
    pred := predictor.New()
    actionEngine := actions.New(clientset, false)
    diagEngine := diagnostics.New(clientset, config)
    autoHealer := diagnostics.NewAutoHealer(diagEngine, false)
    
    // NEW: Start HTTP API Server
    apiServer := api.NewAPIServer(autoHealer, diagEngine, "8080")
    apiServer.Start()
    
    fmt.Println("🚀 AI Monitoring started - COMPLETE SYSTEM ACTIVE")
    fmt.Println("🛠️  Auto-fixing: DNS, disk, network, stuck containers")
    fmt.Println("🌐 Web Dashboard: http://localhost:8080")
    fmt.Println("📊 Status API: http://localhost:8080/status")
    
    for i := 1; ; i++ {
        ctx := context.TODO()
        
        // Standard metrics collection
        metrics, err := col.GetAllPodMetrics(ctx)
        if err != nil {
            fmt.Printf("Error getting metrics: %v\n", err)
            time.Sleep(30 * time.Second)
            continue
        }
        
        // Advanced diagnostics
        stuckContainers, err := diagEngine.DiagnoseStuckContainers(ctx, "")
        if err != nil {
            fmt.Printf("Stuck container diagnostics error: %v\n", err)
        }
        
        containerChecks, err := diagEngine.RunContainerChecks(ctx, "")
        if err != nil {
            fmt.Printf("Container checks error: %v\n", err)
        }
        
        restartPatterns, err := diagEngine.AnalyzeRestartPatterns(ctx, "")
        if err != nil {
            fmt.Printf("Restart analysis error: %v\n", err)
        }
        
        // Execute auto-healing actions
        var healingActions []diagnostics.HealingAction
        if len(containerChecks) > 0 {
            healingActions = autoHealer.HealContainerIssues(ctx, containerChecks)
        }
        
        hasIssues := false
        for _, m := range metrics {
            if m.Restarts > 3 || m.Status != "Running" || m.CPUPercent > 15 || m.MemPercent > 15 {
                hasIssues = true
                break
            }
        }
        
        // Show issues if any diagnostics detected problems
        if len(stuckContainers) > 0 || len(containerChecks) > 0 || len(restartPatterns) > 0 || len(healingActions) > 0 {
            hasIssues = true
        }
        
        if i%20 == 1 || hasIssues {
            fmt.Printf("🔍 Health check [%s]:\n", time.Now().Format("15:04:05"))
            col.PrintStatus()
            
            // Print all diagnostic results
            if len(stuckContainers) > 0 {
                diagEngine.PrintDiagnostics(stuckContainers)
            }
            
            if len(containerChecks) > 0 {
                diagEngine.PrintContainerChecks(containerChecks)
            }
            
            if len(restartPatterns) > 0 {
                diagEngine.PrintRestartAnalysis(restartPatterns)
            }
            
            // Print auto-healing actions
            if len(healingActions) > 0 {
                autoHealer.PrintHealingActions(healingActions)
            }
        } else {
            fmt.Printf("[%s] 🟢 OK (%d until next check) - API: http://localhost:8080\n", 
                time.Now().Format("15:04:05"), 20-(i%20))
        }
        
        // Standard predictions and actions
        pred.UpdateHistory(metrics)
        predictions := pred.PredictIssues(metrics)
        
        if len(predictions) > 0 {
            pred.PrintPredictions(predictions)
            actionEngine.ExecuteActions(predictions)
        }
        
        time.Sleep(30 * time.Second)
    }
}

func createClients() (*kubernetes.Clientset, *metricsclient.Clientset, *rest.Config, error) {
    config, err := rest.InClusterConfig()
    if err != nil {
        var kubeconfig string
        if home := homedir.HomeDir(); home != "" {
            kubeconfig = filepath.Join(home, ".kube", "config")
        }
        config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
        if err != nil {
            return nil, nil, nil, err
        }
    }
    
    clientset, err := kubernetes.NewForConfig(config)
    if err != nil {
        return nil, nil, nil, err
    }
    
    metricsClient, err := metricsclient.NewForConfig(config)
    if err != nil {
        return nil, nil, nil, err
    }
    
    return clientset, metricsClient, config, nil
}

func contains(slice []string, item string) bool {
    for _, s := range slice {
        if s == item {
            return true
        }
    }
    return false
}
