package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"runtime"
	"sync"
	"time"
)

type Snapshot struct {
	Status            string  `json:"status"`
	Attack            bool    `json:"attack"`
	AttackRPS         int     `json:"attack_rps"`
	TargetAttackRPS   int     `json:"target_attack_rps"`
	LegitRPS          int     `json:"legit_rps"`
	Capacity          int     `json:"capacity"`
	Incoming          int     `json:"incoming"`
	EffectiveIncoming int     `json:"effective_incoming"`
	Accepted          int64   `json:"accepted"`
	Dropped           int64   `json:"dropped"`
	Blocked           int64   `json:"blocked"`
	Total             int64   `json:"total"`
	Availability      int     `json:"availability"`
	CPU               int     `json:"cpu"`
	Mitigation        bool    `json:"mitigation"`
	RateLimit         int     `json:"rate_limit"`
	MemoryMB          float64 `json:"memory_mb"`
	Goroutines        int     `json:"goroutines"`
	Updated           string  `json:"updated"`
}

type Lab struct {
	mu                                sync.RWMutex
	attack                            bool
	attackRPS                         int
	targetAttackRPS                   int
	capacity                          int
	legitRPS                          int
	accepted, dropped, blocked, total int64
	availability                      int
	cpu                               int
	mitigation                        bool
	rateLimit                         int // sustained requests/second allowed per source by the modeled token bucket
	bucketBurst                       float64
	bucketTokens                      float64
	effectiveIncoming                 int
	clients                           map[chan Snapshot]bool
}

func NewLab() *Lab {
	l := &Lab{capacity: 120, legitRPS: 35, availability: 100, cpu: 8, rateLimit: 20, bucketBurst: 40, bucketTokens: 40, effectiveIncoming: 35, clients: make(map[chan Snapshot]bool)}
	go l.simulate()
	return l
}

func (l *Lab) snapshotLocked() Snapshot {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	incoming := l.legitRPS + l.attackRPS
	effective := l.effectiveIncoming
	status := "ONLINE"
	if l.availability < 100 {
		status = "DEGRADED"
	}
	if l.availability <= 5 {
		status = "OFFLINE (SIMULATED)"
	}
	return Snapshot{
		Status: status, Attack: l.attack, AttackRPS: l.attackRPS, TargetAttackRPS: l.targetAttackRPS,
		LegitRPS: l.legitRPS, Capacity: l.capacity, Incoming: incoming, EffectiveIncoming: effective,
		Accepted: l.accepted, Dropped: l.dropped, Blocked: l.blocked, Total: l.total,
		Availability: l.availability, CPU: l.cpu, Mitigation: l.mitigation, RateLimit: l.rateLimit,
		MemoryMB: math.Round(float64(m.Alloc)/1024/1024*10) / 10, Goroutines: runtime.NumGoroutine(), Updated: time.Now().Format(time.RFC3339),
	}
}

func (l *Lab) broadcastLocked() {
	s := l.snapshotLocked()
	for ch := range l.clients {
		select {
		case ch <- s:
		default:
		}
	}
}

func approach(current, target, step int) int {
	if current < target {
		return min(current+step, target)
	}
	if current > target {
		return max(current-step, target)
	}
	return current
}

func (l *Lab) simulate() {
	t := time.NewTicker(500 * time.Millisecond)
	defer t.Stop()
	for range t.C {
		l.mu.Lock()
		// Ramp traffic gradually: 50 req/s per second, so the audience can
		// observe the service moving from healthy to degraded to unavailable.
		l.attackRPS = approach(l.attackRPS, l.targetAttackRPS, 25)
		if l.attackRPS == 0 && l.targetAttackRPS == 0 {
			l.attack = false
		}

		rawIncoming := l.legitRPS + l.attackRPS
		effectiveIncoming := rawIncoming
		blockedNow := 0
		if l.mitigation && l.attack {
			// Token Bucket: refill 20 tokens/second, allow a burst of 40,
			// and consume one token per simulated attack request.
			// The normal-user stream is modeled separately and stays unaffected.
			l.bucketTokens = math.Min(l.bucketBurst, l.bucketTokens+float64(l.rateLimit)/2)
			attackRequestsThisTick := l.attackRPS / 2
			allowed := min(attackRequestsThisTick, int(math.Floor(l.bucketTokens)))
			l.bucketTokens -= float64(allowed)
			blockedNow = attackRequestsThisTick - allowed
			effectiveIncoming = l.legitRPS + allowed*2
		} else if !l.mitigation {
			l.bucketTokens = l.bucketBurst
		}
		l.effectiveIncoming = effectiveIncoming

		accepted := min(effectiveIncoming, l.capacity)
		l.total += int64(rawIncoming / 2)
		l.accepted += int64(accepted / 2)
		l.blocked += int64(blockedNow)
		if effectiveIncoming > l.capacity {
			l.dropped += int64((effectiveIncoming - l.capacity) / 2)
		}

		load := float64(effectiveIncoming) / float64(l.capacity)
		targetCPU := int(math.Min(99, 8+load*72))
		targetAvailability := 100
		if load > 0.75 {
			targetAvailability = int(math.Max(0, 100-(load-0.75)*80))
		}
		// Smooth the telemetry as well as the incoming traffic. A 4-point
		// step every 500ms makes degradation visible instead of instantaneous.
		l.cpu = approach(l.cpu, targetCPU, 4)
		l.availability = approach(l.availability, targetAvailability, 4)
		l.broadcastLocked()
		l.mu.Unlock()
	}
}

func main() {
	lab := NewLab()
	mux := http.NewServeMux()
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/user", http.StatusFound)
			return
		}
		switch r.URL.Path {
		case "/monitor":
			http.ServeFile(w, r, "static/monitor.html")
		case "/attacker":
			http.ServeFile(w, r, "static/attacker.html")
		case "/user":
			http.ServeFile(w, r, "static/user.html")
		default:
			http.NotFound(w, r)
		}
	})
	mux.HandleFunc("/api/state", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		lab.mu.RLock()
		s := lab.snapshotLocked()
		lab.mu.RUnlock()
		json.NewEncoder(w).Encode(s)
	})
	mux.HandleFunc("/api/events", func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "stream unsupported", 500)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		ch := make(chan Snapshot, 4)
		lab.mu.Lock()
		lab.clients[ch] = true
		ch <- lab.snapshotLocked()
		lab.mu.Unlock()
		defer func() { lab.mu.Lock(); delete(lab.clients, ch); close(ch); lab.mu.Unlock() }()
		for {
			select {
			case s, ok := <-ch:
				if !ok {
					return
				}
				b, _ := json.Marshal(s)
				fmt.Fprintf(w, "data: %s\n\n", b)
				flusher.Flush()
			case <-r.Context().Done():
				return
			}
		}
	})
	mux.HandleFunc("/api/attack/start", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST required", 405)
			return
		}
		level := 900
		if r.URL.Query().Get("level") == "max" {
			level = 1800
		}
		lab.mu.Lock()
		lab.attack = true
		lab.targetAttackRPS = level
		lab.broadcastLocked()
		lab.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/api/attack/stop", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST required", 405)
			return
		}
		lab.mu.Lock()
		lab.targetAttackRPS = 0
		lab.broadcastLocked()
		lab.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/api/mitigation/toggle", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST required", 405)
			return
		}
		lab.mu.Lock()
		lab.mitigation = !lab.mitigation
		if lab.mitigation {
			lab.bucketTokens = lab.bucketBurst
		}
		lab.broadcastLocked()
		lab.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/api/reset", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST required", 405)
			return
		}
		lab.mu.Lock()
		lab.attack = false
		lab.attackRPS = 0
		lab.targetAttackRPS = 0
		lab.accepted = 0
		lab.dropped = 0
		lab.blocked = 0
		lab.total = 0
		lab.availability = 100
		lab.cpu = 8
		lab.mitigation = false
		lab.bucketTokens = lab.bucketBurst
		lab.effectiveIncoming = lab.legitRPS
		lab.broadcastLocked()
		lab.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	})
	addr := ":8080"
	log.Printf("DDoS Lab running at http://localhost%s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
