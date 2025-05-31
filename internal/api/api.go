package api

import (
    "encoding/json"
    "fmt"
    "net/http"
    "time"
    
    "k8s-healer/internal/diagnostics"
)

type APIServer struct {
    autoHealer   *diagnostics.AutoHealer
    diagEngine   *diagnostics.DiagnosticsEngine
    port         string
}

type StatusResponse struct {
    Status        string                           `json:"status"`
    Timestamp     time.Time                       `json:"timestamp"`
    TotalActions  int                             `json:"total_actions"`
    RecentActions []diagnostics.HealingAction     `json:"recent_actions"`
    SystemHealth  string                          `json:"system_health"`
}

func NewAPIServer(autoHealer *diagnostics.AutoHealer, diagEngine *diagnostics.DiagnosticsEngine, port string) *APIServer {
    return &APIServer{
        autoHealer: autoHealer,
        diagEngine: diagEngine,
        port:       port,
    }
}

func (s *APIServer) Start() {
    http.HandleFunc("/", s.handleRoot)
    http.HandleFunc("/status", s.handleStatus)
    http.HandleFunc("/actions", s.handleActions)
    http.HandleFunc("/health", s.handleHealth)
    
    fmt.Printf("🌐 API Server starting on port %s\n", s.port)
    fmt.Printf("📊 Access at: http://localhost:%s/status\n", s.port)
    
    go func() {
        if err := http.ListenAndServe(":"+s.port, nil); err != nil {
            fmt.Printf("API Server error: %v\n", err)
        }
    }()
}

func (s *APIServer) handleRoot(w http.ResponseWriter, r *http.Request) {
    html := `<!DOCTYPE html>
<html>
<head>
    <title>K8s AI Healer Dashboard</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; background: #f5f5f5; }
        .container { max-width: 1200px; margin: 0 auto; }
        .header { background: #2196F3; color: white; padding: 20px; border-radius: 8px; text-align: center; }
        .card { background: white; margin: 20px 0; padding: 20px; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        .nav { margin: 20px 0; }
        .nav a { margin-right: 20px; padding: 10px 20px; background: #2196F3; color: white; text-decoration: none; border-radius: 4px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>🤖 K8s AI Healer Dashboard</h1>
            <p>Advanced Kubernetes Infrastructure Healing System</p>
        </div>
        <div class="nav">
            <a href="/status">System Status</a>
            <a href="/actions">Healing Actions</a>
            <a href="/health">Health Check</a>
        </div>
        <div class="card">
            <h2>🛠️ System Overview</h2>
            <p>The K8s AI Healer detects and fixes infrastructure issues that Kubernetes might miss:</p>
            <ul>
                <li>🔍 Stuck Container Detection</li>
                <li>🌐 Network Connectivity Issues</li>
                <li>💾 Disk Space Management</li>
                <li>🔄 Restart Pattern Analysis</li>
                <li>🚀 Automatic Healing</li>
            </ul>
        </div>
    </div>
</body>
</html>`
    
    w.Header().Set("Content-Type", "text/html")
    w.Write([]byte(html))
}

func (s *APIServer) handleStatus(w http.ResponseWriter, r *http.Request) {
    history := s.autoHealer.GetHealingHistory()
    
    var recentActions []diagnostics.HealingAction
    if len(history) > 0 {
        start := 0
        if len(history) > 10 {
            start = len(history) - 10
        }
        recentActions = history[start:]
    }
    
    systemHealth := "HEALTHY"
    if len(recentActions) > 0 {
        criticalCount := 0
        for _, action := range recentActions {
            if action.ActionType == "RESTART_POD_NETWORK" || action.Status == "FAILED" {
                criticalCount++
            }
        }
        if criticalCount > 5 {
            systemHealth = "CRITICAL"
        } else if criticalCount > 0 {
            systemHealth = "WARNING"
        }
    }
    
    response := StatusResponse{
        Status:        "ACTIVE",
        Timestamp:     time.Now(),
        TotalActions:  len(history),
        RecentActions: recentActions,
        SystemHealth:  systemHealth,
    }
    
    w.Header().Set("Content-Type", "application/json")
    w.Header().Set("Access-Control-Allow-Origin", "*")
    
    json.NewEncoder(w).Encode(response)
}

func (s *APIServer) handleActions(w http.ResponseWriter, r *http.Request) {
    history := s.autoHealer.GetHealingHistory()
    
    w.Header().Set("Content-Type", "application/json")
    w.Header().Set("Access-Control-Allow-Origin", "*")
    
    json.NewEncoder(w).Encode(map[string]interface{}{
        "total_actions": len(history),
        "actions":       history,
    })
}

func (s *APIServer) handleHealth(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    w.Header().Set("Access-Control-Allow-Origin", "*")
    
    json.NewEncoder(w).Encode(map[string]interface{}{
        "status":    "UP",
        "timestamp": time.Now(),
        "service":   "k8s-ai-healer",
        "version":   "3.0",
    })
}
