package collector

import (
    "context"
    "fmt"
    "strings"
    "time"

    "k8s.io/client-go/kubernetes"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    metricsclient "k8s.io/metrics/pkg/client/clientset/versioned"
    "k8s.io/apimachinery/pkg/api/resource"
)

type Collector struct {
    clientset     *kubernetes.Clientset
    metricsClient *metricsclient.Clientset
}

type PodMetrics struct {
    Name         string
    Namespace    string
    CPUUsage     string
    MemUsage     string
    CPUPercent   float64
    MemPercent   float64
    Status       string
    Restarts     int32
    Age          time.Duration
    NodeName     string
}

type NodeMetrics struct {
    Name         string
    CPUUsage     string
    MemUsage     string
    CPUPercent   float64
    MemPercent   float64
    PodCount     int
}

func New(clientset *kubernetes.Clientset, metricsClient *metricsclient.Clientset) *Collector {
    return &Collector{
        clientset:     clientset,
        metricsClient: metricsClient,
    }
}

func (c *Collector) GetAllPodMetrics(ctx context.Context) ([]PodMetrics, error) {
    // Get pods from K8s API (NO kubectl!)
    pods, err := c.clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
    if err != nil {
        return nil, fmt.Errorf("failed to get pods: %v", err)
    }

    // Get pod metrics from metrics API (NO kubectl dependency!)
    podMetricsAPI, err := c.metricsClient.MetricsV1beta1().PodMetricses("").List(ctx, metav1.ListOptions{})
    if err != nil {
        fmt.Printf("Warning: Metrics API not available: %v\n", err)
        // Continue without metrics - better than failing
    }

    // Create metrics map for fast lookup
    metricsMap := make(map[string]map[string]resource.Quantity)
    if podMetricsAPI != nil {
        for _, podMetric := range podMetricsAPI.Items {
            key := fmt.Sprintf("%s/%s", podMetric.Namespace, podMetric.Name)
            containerMetrics := make(map[string]resource.Quantity)
            
            for _, container := range podMetric.Containers {
                if cpu, exists := container.Usage["cpu"]; exists {
                    containerMetrics["cpu"] = cpu
                }
                if memory, exists := container.Usage["memory"]; exists {
                    containerMetrics["memory"] = memory
                }
            }
            metricsMap[key] = containerMetrics
        }
    }

    var metrics []PodMetrics
    
    for _, pod := range pods.Items {
        // Skip system pods
        if strings.Contains(pod.Namespace, "kube-") || 
           strings.Contains(pod.Namespace, "healer-") {
            continue
        }

        var restarts int32
        for _, cs := range pod.Status.ContainerStatuses {
            restarts += cs.RestartCount
        }

        age := time.Since(pod.CreationTimestamp.Time)
        podKey := fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)

        podMetric := PodMetrics{
            Name:      pod.Name,
            Namespace: pod.Namespace,
            Status:    string(pod.Status.Phase),
            Restarts:  restarts,
            Age:       age,
            NodeName:  pod.Spec.NodeName,
            CPUUsage:  "0m",
            MemUsage:  "0Mi",
            CPUPercent: 0.0,
            MemPercent: 0.0,
        }

        // Get actual metrics if available
        if containerMetrics, exists := metricsMap[podKey]; exists {
            if cpu, hasCPU := containerMetrics["cpu"]; hasCPU {
                podMetric.CPUUsage = cpu.String()
                // Convert to percentage (simplified)
                cpuMilli := float64(cpu.MilliValue())
                podMetric.CPUPercent = cpuMilli / 10.0 // Rough percentage
            }
            if memory, hasMem := containerMetrics["memory"]; hasMem {
                podMetric.MemUsage = memory.String()
                // Convert to percentage (simplified - assumes 1Gi limit)
                memBytes := float64(memory.Value())
                podMetric.MemPercent = (memBytes / (1024 * 1024 * 1024)) * 100
            }
        }

        metrics = append(metrics, podMetric)
    }

    return metrics, nil
}

func (c *Collector) GetNodeMetrics(ctx context.Context) ([]NodeMetrics, error) {
    nodes, err := c.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
    if err != nil {
        return nil, fmt.Errorf("failed to get nodes: %v", err)
    }

    nodeMetricsAPI, err := c.metricsClient.MetricsV1beta1().NodeMetricses().List(ctx, metav1.ListOptions{})
    if err != nil {
        fmt.Printf("Warning: Node metrics not available: %v\n", err)
        nodeMetricsAPI = nil
    }

    metricsMap := make(map[string]map[string]resource.Quantity)
    if nodeMetricsAPI != nil {
        for _, nodeMetric := range nodeMetricsAPI.Items {
            containerMetrics := make(map[string]resource.Quantity)
            if cpu, exists := nodeMetric.Usage["cpu"]; exists {
                containerMetrics["cpu"] = cpu
            }
            if memory, exists := nodeMetric.Usage["memory"]; exists {
                containerMetrics["memory"] = memory
            }
            metricsMap[nodeMetric.Name] = containerMetrics
        }
    }

    var nodeMetrics []NodeMetrics
    for _, node := range nodes.Items {
        metric := NodeMetrics{
            Name:       node.Name,
            CPUUsage:   "0m",
            MemUsage:   "0Mi",
            CPUPercent: 0.0,
            MemPercent: 0.0,
            PodCount:   0,
        }

        if containerMetrics, exists := metricsMap[node.Name]; exists {
            if cpu, hasCPU := containerMetrics["cpu"]; hasCPU {
                metric.CPUUsage = cpu.String()
                // Calculate percentage based on node capacity
                if capacity, hasCapacity := node.Status.Capacity["cpu"]; hasCapacity {
                    cpuUsed := float64(cpu.MilliValue())
                    cpuTotal := float64(capacity.MilliValue())
                    metric.CPUPercent = (cpuUsed / cpuTotal) * 100
                }
            }
            if memory, hasMem := containerMetrics["memory"]; hasMem {
                metric.MemUsage = memory.String()
                if capacity, hasCapacity := node.Status.Capacity["memory"]; hasCapacity {
                    memUsed := float64(memory.Value())
                    memTotal := float64(capacity.Value())
                    metric.MemPercent = (memUsed / memTotal) * 100
                }
            }
        }

        // Count pods on this node
        pods, err := c.clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{
            FieldSelector: fmt.Sprintf("spec.nodeName=%s", node.Name),
        })
        if err == nil {
            metric.PodCount = len(pods.Items)
        }

        nodeMetrics = append(nodeMetrics, metric)
    }

    return nodeMetrics, nil
}

func (c *Collector) PrintStatus() {
    ctx := context.TODO()
    
    metrics, err := c.GetAllPodMetrics(ctx)
    if err != nil {
        fmt.Printf("Failed to get pod metrics: %v\n", err)
        return
    }

    nodeMetrics, err := c.GetNodeMetrics(ctx)
    if err != nil {
        fmt.Printf("Failed to get node metrics: %v\n", err)
    }

    fmt.Printf("=== INFRASTRUCTURE HEALTH STATUS ===\n")
    
    // Print node status
    if len(nodeMetrics) > 0 {
        fmt.Printf("🖥️  NODES:\n")
        for _, node := range nodeMetrics {
            status := "✅ HEALTHY"
            if node.CPUPercent > 80 || node.MemPercent > 80 {
                status = "⚠️  HIGH LOAD"
            }
            fmt.Printf("  Node: %s - %s (CPU: %.1f%%, Mem: %.1f%%, Pods: %d)\n", 
                node.Name, status, node.CPUPercent, node.MemPercent, node.PodCount)
        }
        fmt.Printf("\n")
    }

    // Print pod status
    fmt.Printf("🚀 PODS:\n")
    for _, metric := range metrics {
        status := "✅ HEALTHY"
        if metric.Restarts > 5 {
            status = "⚠️  HIGH RESTARTS"
        }
        if metric.Status != "Running" {
            status = "❌ NOT RUNNING"
        }
        if metric.CPUPercent > 80 || metric.MemPercent > 80 {
            status = "🔥 HIGH RESOURCE USAGE"
        }

        fmt.Printf("  Pod: %s/%s - %s\n", metric.Namespace, metric.Name, status)
        fmt.Printf("    Resources: CPU: %.1f%% (%s), Memory: %.1f%% (%s)\n", 
            metric.CPUPercent, metric.CPUUsage, metric.MemPercent, metric.MemUsage)
        fmt.Printf("    Restarts: %d, Age: %v, Node: %s\n\n", 
            metric.Restarts, metric.Age.Round(time.Second), metric.NodeName)
    }
    fmt.Printf("====================================\n\n")
}
