package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
)

type AutoScaler struct {
	redis           *redis.Client
	config          *Config
	currentReplicas int
	mutex           sync.RWMutex
	metrics         *Metrics
}

type Config struct {
	MinReplicas     int     `json:"min_replicas"`
	MaxReplicas     int     `json:"max_replicas"`
	TargetCPU       float64 `json:"target_cpu_percent"`
	TargetMemory    float64 `json:"target_memory_percent"`
	TargetRPS       int     `json:"target_requests_per_second"`
	ScaleUpCooldown time.Duration `json:"scale_up_cooldown"`
	ScaleDownCooldown time.Duration `json:"scale_down_cooldown"`
	CheckInterval   time.Duration `json:"check_interval"`
}

type Metrics struct {
	CurrentRPS      int       `json:"current_rps"`
	AvgResponseTime float64   `json:"avg_response_time_ms"`
	ActiveConns     int       `json:"active_connections"`
	NodeCount       int       `json:"available_nodes"`
	CPUUsage        float64   `json:"cpu_usage_percent"`
	MemoryUsage     float64   `json:"memory_usage_percent"`
	QueueDepth      int       `json:"queue_depth"`
	Replicas        int       `json:"current_replicas"`
	LastScaleTime   time.Time `json:"last_scale_time"`
	LastScaleAction string    `json:"last_scale_action"`
}

func NewAutoScaler() *AutoScaler {
	redisAddr := os.Getenv("REDIS_URL")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	config := &Config{
		MinReplicas:       1,
		MaxReplicas:       5,
		TargetCPU:         70.0,
		TargetMemory:      80.0,
		TargetRPS:         1000,
		ScaleUpCooldown:   2 * time.Minute,
		ScaleDownCooldown: 5 * time.Minute,
		CheckInterval:     30 * time.Second,
	}

	// Override from env
	if v := os.Getenv("MIN_REPLICAS"); v != "" {
		config.MinReplicas, _ = strconv.Atoi(v)
	}
	if v := os.Getenv("MAX_REPLICAS"); v != "" {
		config.MaxReplicas, _ = strconv.Atoi(v)
	}
	if v := os.Getenv("TARGET_RPS"); v != "" {
		config.TargetRPS, _ = strconv.Atoi(v)
	}

	return &AutoScaler{
		redis:           rdb,
		config:          config,
		currentReplicas: 1,
		metrics:         &Metrics{Replicas: 1},
	}
}

func (as *AutoScaler) Start() {
	log.Println("AutoScaler started")
	
	// Start metrics collection
	go as.collectMetrics()
	
	// Start scaling loop
	go as.scalingLoop()
	
	// Start HTTP API
	as.startAPI()
}

func (as *AutoScaler) collectMetrics() {
	ctx := context.Background()
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		as.mutex.Lock()
		
		// Get RPS from Redis (gateway publishes this)
		rps, _ := as.redis.Get(ctx, "iploop:metrics:rps").Int()
		as.metrics.CurrentRPS = rps
		
		// Get active connections
		conns, _ := as.redis.Get(ctx, "iploop:metrics:active_connections").Int()
		as.metrics.ActiveConns = conns
		
		// Get avg response time
		respTime, _ := as.redis.Get(ctx, "iploop:metrics:avg_response_time").Float64()
		as.metrics.AvgResponseTime = respTime
		
		// Get available nodes
		nodes, _ := as.redis.SCard(ctx, "iploop:nodes:online").Result()
		as.metrics.NodeCount = int(nodes)
		
		// Get queue depth
		queue, _ := as.redis.LLen(ctx, "iploop:request_queue").Result()
		as.metrics.QueueDepth = int(queue)
		
		// Get container stats (simplified - would use Docker API in production)
		as.metrics.CPUUsage = as.getContainerCPU()
		as.metrics.MemoryUsage = as.getContainerMemory()
		
		as.mutex.Unlock()
		
		// Publish metrics
		metricsJSON, _ := json.Marshal(as.metrics)
		as.redis.Set(ctx, "iploop:autoscaler:metrics", metricsJSON, 60*time.Second)
	}
}

func (as *AutoScaler) getContainerCPU() float64 {
	cmd := exec.Command("sh", "-c", 
		"docker stats --no-stream --format '{{.CPUPerc}}' iploop-proxy-gateway 2>/dev/null | tr -d '%' | head -1")
	out, err := cmd.Output()
	if err != nil {
		return 0
	}
	cpu, _ := strconv.ParseFloat(string(out[:len(out)-1]), 64)
	return cpu
}

func (as *AutoScaler) getContainerMemory() float64 {
	cmd := exec.Command("sh", "-c",
		"docker stats --no-stream --format '{{.MemPerc}}' iploop-proxy-gateway 2>/dev/null | tr -d '%' | head -1")
	out, err := cmd.Output()
	if err != nil {
		return 0
	}
	mem, _ := strconv.ParseFloat(string(out[:len(out)-1]), 64)
	return mem
}

func (as *AutoScaler) scalingLoop() {
	var lastScaleUp, lastScaleDown time.Time
	ticker := time.NewTicker(as.config.CheckInterval)
	defer ticker.Stop()

	for range ticker.C {
		as.mutex.RLock()
		metrics := *as.metrics
		currentReplicas := as.currentReplicas
		as.mutex.RUnlock()

		decision := as.evaluateScaling(metrics, currentReplicas)
		
		switch decision {
		case "scale_up":
			if time.Since(lastScaleUp) > as.config.ScaleUpCooldown {
				if err := as.scaleUp(); err != nil {
					log.Printf("Scale up failed: %v", err)
				} else {
					lastScaleUp = time.Now()
					as.mutex.Lock()
					as.metrics.LastScaleTime = lastScaleUp
					as.metrics.LastScaleAction = "scale_up"
					as.mutex.Unlock()
				}
			}
		case "scale_down":
			if time.Since(lastScaleDown) > as.config.ScaleDownCooldown {
				if err := as.scaleDown(); err != nil {
					log.Printf("Scale down failed: %v", err)
				} else {
					lastScaleDown = time.Now()
					as.mutex.Lock()
					as.metrics.LastScaleTime = lastScaleDown
					as.metrics.LastScaleAction = "scale_down"
					as.mutex.Unlock()
				}
			}
		}
	}
}

func (as *AutoScaler) evaluateScaling(metrics Metrics, currentReplicas int) string {
	// Scale up conditions
	scaleUp := false
	scaleDown := false

	// High CPU
	if metrics.CPUUsage > as.config.TargetCPU {
		log.Printf("High CPU: %.1f%% > %.1f%%", metrics.CPUUsage, as.config.TargetCPU)
		scaleUp = true
	}

	// High Memory
	if metrics.MemoryUsage > as.config.TargetMemory {
		log.Printf("High Memory: %.1f%% > %.1f%%", metrics.MemoryUsage, as.config.TargetMemory)
		scaleUp = true
	}

	// High RPS
	rpsPerReplica := metrics.CurrentRPS / max(currentReplicas, 1)
	if rpsPerReplica > as.config.TargetRPS {
		log.Printf("High RPS per replica: %d > %d", rpsPerReplica, as.config.TargetRPS)
		scaleUp = true
	}

	// Queue building up
	if metrics.QueueDepth > 100 {
		log.Printf("Queue depth high: %d", metrics.QueueDepth)
		scaleUp = true
	}

	// Scale down conditions (conservative)
	if metrics.CPUUsage < as.config.TargetCPU*0.3 &&
		metrics.MemoryUsage < as.config.TargetMemory*0.3 &&
		rpsPerReplica < as.config.TargetRPS/3 &&
		metrics.QueueDepth == 0 {
		scaleDown = true
	}

	// Apply limits
	if scaleUp && currentReplicas < as.config.MaxReplicas {
		return "scale_up"
	}
	if scaleDown && currentReplicas > as.config.MinReplicas {
		return "scale_down"
	}

	return "no_change"
}

func (as *AutoScaler) scaleUp() error {
	as.mutex.Lock()
	defer as.mutex.Unlock()

	newCount := as.currentReplicas + 1
	if newCount > as.config.MaxReplicas {
		return fmt.Errorf("already at max replicas")
	}

	log.Printf("Scaling UP: %d -> %d replicas", as.currentReplicas, newCount)
	
	cmd := exec.Command("docker", "compose", "-f", "/root/clawd-secure/iploop-platform/docker-compose.yml",
		"up", "-d", "--scale", fmt.Sprintf("proxy-gateway=%d", newCount), "--no-recreate")
	
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker scale failed: %v", err)
	}

	as.currentReplicas = newCount
	as.metrics.Replicas = newCount
	
	// Notify via Redis
	as.redis.Publish(context.Background(), "iploop:autoscaler:events", 
		fmt.Sprintf(`{"action":"scale_up","replicas":%d}`, newCount))
	
	return nil
}

func (as *AutoScaler) scaleDown() error {
	as.mutex.Lock()
	defer as.mutex.Unlock()

	newCount := as.currentReplicas - 1
	if newCount < as.config.MinReplicas {
		return fmt.Errorf("already at min replicas")
	}

	log.Printf("Scaling DOWN: %d -> %d replicas", as.currentReplicas, newCount)
	
	cmd := exec.Command("docker", "compose", "-f", "/root/clawd-secure/iploop-platform/docker-compose.yml",
		"up", "-d", "--scale", fmt.Sprintf("proxy-gateway=%d", newCount), "--no-recreate")
	
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker scale failed: %v", err)
	}

	as.currentReplicas = newCount
	as.metrics.Replicas = newCount
	
	// Notify via Redis
	as.redis.Publish(context.Background(), "iploop:autoscaler:events",
		fmt.Sprintf(`{"action":"scale_down","replicas":%d}`, newCount))
	
	return nil
}

func (as *AutoScaler) startAPI() {
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy"}`))
	})

	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		as.mutex.RLock()
		defer as.mutex.RUnlock()
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(as.metrics)
	})

	http.HandleFunc("/config", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(as.config)
	})

	http.HandleFunc("/scale", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "POST required", http.StatusMethodNotAllowed)
			return
		}
		
		action := r.URL.Query().Get("action")
		var err error
		
		switch action {
		case "up":
			err = as.scaleUp()
		case "down":
			err = as.scaleDown()
		default:
			http.Error(w, "action must be 'up' or 'down'", http.StatusBadRequest)
			return
		}
		
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":  true,
			"replicas": as.currentReplicas,
		})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8090"
	}
	
	log.Printf("AutoScaler API listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func main() {
	as := NewAutoScaler()
	as.Start()
}
