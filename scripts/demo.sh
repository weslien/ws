#!/usr/bin/env bash
#
# ws Multi-Agent Demo — Calculator API
#
# Demonstrates a 3-wave multi-agent workflow using ws:
#   Wave 0: Planner creates 6 workspaces from a base repo
#   Wave 1: 4 agents work in parallel on independent features
#   Wave 2: 2 agents branch from wave-1 agents and add dependent features
#   Wave 3: Consolidator merges all 6 layers into one compilable binary
#
# Requires: ws CLI (https://github.com/weslien/ws), go, curl
#
set -euo pipefail

WS="${WS:-ws}"
PORT="${PORT:-8090}"

echo "╔══════════════════════════════════════════════════════════╗"
echo "║  ws Multi-Agent Demo — Calculator API                    ║"
echo "║  7 agents · 3 waves · 9 layers · 1 merged binary         ║"
echo "╚══════════════════════════════════════════════════════════╝"
echo ""

# ── 0. Create base repo ─────────────────────────────────────────────────────
echo "▶ Wave 0: Creating base repo..."
DEMO_REPO="/tmp/ws-demo-repo"
rm -rf "$DEMO_REPO"
mkdir -p "$DEMO_REPO"
cd "$DEMO_REPO"
git init -q

cat > main.go << 'GOEOF'
package main

import (
	"net/http"
)

func handleAdd(w http.ResponseWriter, r *http.Request)      { /* stub */ }
func handleSubtract(w http.ResponseWriter, r *http.Request)  { /* stub */ }
func handleMultiply(w http.ResponseWriter, r *http.Request)  { /* stub */ }
func handleDivide(w http.ResponseWriter, r *http.Request)    { /* stub */ }

func main() {
	http.HandleFunc("/add", handleAdd)
	http.HandleFunc("/subtract", handleSubtract)
	http.HandleFunc("/multiply", handleMultiply)
	http.HandleFunc("/divide", handleDivide)
	http.ListenAndServe(":${PORT}", nil)
}
GOEOF

# Replace the port placeholder
sed -i "s/\${PORT}/$PORT/" main.go
git add -A
git commit -q -m "base: calculator API skeleton"

echo "  ✓ Base repo at $DEMO_REPO"
echo ""

# ── 1. Create base layer + 6 workspaces ──────────────────────────────────────
echo "▶ Wave 1: Creating 6 workspaces from base..."
rm -rf ~/.ws
mkdir -p ~/.ws/{layers,workspaces,uppers,workdirs,meta}

$WS get base:"$DEMO_REPO" --name=base
$WS keep base --message="base: calculator API skeleton"
$WS drop base
BASE_LAYER=$($WS layer ls | grep "base:" | awk '{print $1}')
echo "  ✓ Base layer: $BASE_LAYER"

for i in 1 2 3 4 5 6; do
	$WS get layer:"$BASE_LAYER" --name="agent-$i"
done
echo "  ✓ 6 workspaces created"
echo ""

# ── 2. Agent implementations ─────────────────────────────────────────────────
echo "▶ Wave 1: Agents implementing features in parallel..."

# Agent 1: add + subtract
cat > ~/.ws/workspaces/agent-1/main.go << 'GOEOF'
package main

import (
	"encoding/json"
	"net/http"
	"strconv"
)

func handleAdd(w http.ResponseWriter, r *http.Request) {
	a, _ := strconv.Atoi(r.URL.Query().Get("a"))
	b, _ := strconv.Atoi(r.URL.Query().Get("b"))
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{"result": a + b})
}

func handleSubtract(w http.ResponseWriter, r *http.Request) {
	a, _ := strconv.Atoi(r.URL.Query().Get("a"))
	b, _ := strconv.Atoi(r.URL.Query().Get("b"))
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{"result": a - b})
}

func handleMultiply(w http.ResponseWriter, r *http.Request)  { /* stub */ }
func handleDivide(w http.ResponseWriter, r *http.Request)    { /* stub */ }

func main() {
	http.HandleFunc("/add", handleAdd)
	http.HandleFunc("/subtract", handleSubtract)
	http.HandleFunc("/multiply", handleMultiply)
	http.HandleFunc("/divide", handleDivide)
	http.ListenAndServe(":8090", nil)
}
GOEOF

$WS run agent-1 -- go build -o /dev/null main.go
LAYER1=$($WS keep agent-1 --message="agent-1: add+subtract" | grep "kept layer" | awk '{print $3}')
echo "  ✓ agent-1: add+subtract → $LAYER1"

# Agent 2: multiply + divide
cat > ~/.ws/workspaces/agent-2/main.go << 'GOEOF'
package main

import (
	"encoding/json"
	"net/http"
	"strconv"
)

func handleAdd(w http.ResponseWriter, r *http.Request)      { /* stub */ }
func handleSubtract(w http.ResponseWriter, r *http.Request)  { /* stub */ }

func handleMultiply(w http.ResponseWriter, r *http.Request) {
	a, _ := strconv.Atoi(r.URL.Query().Get("a"))
	b, _ := strconv.Atoi(r.URL.Query().Get("b"))
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{"result": a * b})
}

func handleDivide(w http.ResponseWriter, r *http.Request) {
	a, _ := strconv.Atoi(r.URL.Query().Get("a"))
	b, _ := strconv.Atoi(r.URL.Query().Get("b"))
	if b == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "division by zero"})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{"result": a / b})
}

func main() {
	http.HandleFunc("/add", handleAdd)
	http.HandleFunc("/subtract", handleSubtract)
	http.HandleFunc("/multiply", handleMultiply)
	http.HandleFunc("/divide", handleDivide)
	http.ListenAndServe(":8090", nil)
}
GOEOF

$WS run agent-2 -- go build -o /dev/null main.go
LAYER2=$($WS keep agent-2 --message="agent-2: multiply+divide" | grep "kept layer" | awk '{print $3}')
echo "  ✓ agent-2: multiply+divide → $LAYER2"

# Agent 4: /health endpoint
cat > ~/.ws/workspaces/agent-4/main.go << 'GOEOF'
package main

import (
	"encoding/json"
	"net/http"
)

func handleAdd(w http.ResponseWriter, r *http.Request)      { /* stub */ }
func handleSubtract(w http.ResponseWriter, r *http.Request)  { /* stub */ }
func handleMultiply(w http.ResponseWriter, r *http.Request)  { /* stub */ }
func handleDivide(w http.ResponseWriter, r *http.Request)    { /* stub */ }

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status": "ok"}`))
}

func main() {
	http.HandleFunc("/add", handleAdd)
	http.HandleFunc("/subtract", handleSubtract)
	http.HandleFunc("/multiply", handleMultiply)
	http.HandleFunc("/divide", handleDivide)
	http.HandleFunc("/health", handleHealth)
	http.ListenAndServe(":8090", nil)
}
GOEOF

$WS run agent-4 -- go build -o /dev/null main.go
LAYER4=$($WS keep agent-4 --message="agent-4: /health endpoint" | grep "kept layer" | awk '{print $3}')
echo "  ✓ agent-4: /health → $LAYER4"

# Agent 6: /metrics endpoint
cat > ~/.ws/workspaces/agent-6/main.go << 'GOEOF'
package main

import (
	"encoding/json"
	"net/http"
	"sync"
)

var (
	metricsMu     sync.Mutex
	requestCounts = map[string]int{}
)

func handleAdd(w http.ResponseWriter, r *http.Request)      { /* stub */ }
func handleSubtract(w http.ResponseWriter, r *http.Request)  { /* stub */ }
func handleMultiply(w http.ResponseWriter, r *http.Request)  { /* stub */ }
func handleDivide(w http.ResponseWriter, r *http.Request)    { /* stub */ }

func countMiddleware(path string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		metricsMu.Lock()
		requestCounts[path]++
		metricsMu.Unlock()
		next(w, r)
	}
}

func handleMetrics(w http.ResponseWriter, r *http.Request) {
	metricsMu.Lock()
	counts := make(map[string]int, len(requestCounts))
	for k, v := range requestCounts {
		counts[k] = v
	}
	metricsMu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]map[string]int{"endpoints": counts})
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/add", countMiddleware("/add", handleAdd))
	mux.HandleFunc("/subtract", countMiddleware("/subtract", handleSubtract))
	mux.HandleFunc("/multiply", countMiddleware("/multiply", handleMultiply))
	mux.HandleFunc("/divide", countMiddleware("/divide", handleDivide))
	mux.HandleFunc("/metrics", handleMetrics)
	http.ListenAndServe(":8090", mux)
}
GOEOF

$WS run agent-6 -- go build -o /dev/null main.go
LAYER6=$($WS keep agent-6 --message="agent-6: /metrics endpoint" | grep "kept layer" | awk '{print $3}')
echo "  ✓ agent-6: /metrics → $LAYER6"
echo ""

# ── 3. Wave 2: Branch from wave 1 ─────────────────────────────────────────────
echo "▶ Wave 2: Branching dependent agents from wave 1..."

$WS get ws:agent-1 --name=agent-3 --force
$WS get ws:agent-2 --name=agent-5 --force

# Agent 3: input validation (from agent-1)
cat > ~/.ws/workspaces/agent-3/main.go << 'GOEOF'
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

func validateParams(w http.ResponseWriter, r *http.Request) (int, int, bool) {
	aStr := r.URL.Query().Get("a")
	bStr := r.URL.Query().Get("b")
	if aStr == "" {
		writeError(w, "missing parameter 'a'")
		return 0, 0, false
	}
	if bStr == "" {
		writeError(w, "missing parameter 'b'")
		return 0, 0, false
	}
	a, err := strconv.Atoi(aStr)
	if err != nil {
		writeError(w, fmt.Sprintf("invalid integer: '%s'", aStr))
		return 0, 0, false
	}
	b, err := strconv.Atoi(bStr)
	if err != nil {
		writeError(w, fmt.Sprintf("invalid integer: '%s'", bStr))
		return 0, 0, false
	}
	return a, b, true
}

func writeError(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func writeResult(w http.ResponseWriter, v int) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{"result": v})
}

func handleAdd(w http.ResponseWriter, r *http.Request) {
	a, b, ok := validateParams(w, r)
	if !ok { return }
	writeResult(w, a+b)
}

func handleSubtract(w http.ResponseWriter, r *http.Request) {
	a, b, ok := validateParams(w, r)
	if !ok { return }
	writeResult(w, a-b)
}

func handleMultiply(w http.ResponseWriter, r *http.Request)  { /* stub */ }
func handleDivide(w http.ResponseWriter, r *http.Request)    { /* stub */ }

func main() {
	http.HandleFunc("/add", handleAdd)
	http.HandleFunc("/subtract", handleSubtract)
	http.HandleFunc("/multiply", handleMultiply)
	http.HandleFunc("/divide", handleDivide)
	http.ListenAndServe(":8090", nil)
}
GOEOF

$WS run agent-3 -- go build -o /dev/null main.go
LAYER3=$($WS keep agent-3 --message="agent-3: input validation" | grep "kept layer" | awk '{print $3}')
echo "  ✓ agent-3: validation (from agent-1) → $LAYER3"

# Agent 5: structured logging (from agent-2)
cat > ~/.ws/workspaces/agent-5/main.go << 'GOEOF'
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"
)

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		log.Printf("method=%s path=%s status=%d duration=%.3fms",
			r.Method, r.URL.Path, rec.status,
			float64(time.Since(start).Microseconds())/1000.0)
	})
}

func handleAdd(w http.ResponseWriter, r *http.Request)      { /* stub */ }
func handleSubtract(w http.ResponseWriter, r *http.Request)  { /* stub */ }

func handleMultiply(w http.ResponseWriter, r *http.Request) {
	a, _ := strconv.Atoi(r.URL.Query().Get("a"))
	b, _ := strconv.Atoi(r.URL.Query().Get("b"))
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{"result": a * b})
}

func handleDivide(w http.ResponseWriter, r *http.Request) {
	a, _ := strconv.Atoi(r.URL.Query().Get("a"))
	b, _ := strconv.Atoi(r.URL.Query().Get("b"))
	if b == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "division by zero"})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{"result": a / b})
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/add", handleAdd)
	mux.HandleFunc("/subtract", handleSubtract)
	mux.HandleFunc("/multiply", handleMultiply)
	mux.HandleFunc("/divide", handleDivide)
	http.ListenAndServe(":8090", loggingMiddleware(mux))
}
GOEOF

$WS run agent-5 -- go build -o /dev/null main.go
LAYER5=$($WS keep agent-5 --message="agent-5: structured logging" | grep "kept layer" | awk '{print $3}')
echo "  ✓ agent-5: logging (from agent-2) → $LAYER5"
echo ""

# ── 4. Wave 3: Consolidate ────────────────────────────────────────────────────
echo "▶ Wave 3: Consolidating all 6 agents into one binary..."

$WS get layer:"$LAYER3" --name=final --force

# The consolidator would use `ws layer cat` to read each agent's version.
# For this scripted demo, we write the merged file directly.
cat > ~/.ws/workspaces/final/main.go << 'GOEOF'
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// --- Metrics state (agent-6) ---
var (
	metricsMu     sync.Mutex
	requestCounts = map[string]int{}
)

// --- Middleware (agent-5 logging, agent-6 counting) ---
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		log.Printf("method=%s path=%s query=%q status=%d duration=%.3fms",
			r.Method, r.URL.Path, r.URL.RawQuery, rec.status,
			float64(time.Since(start).Microseconds())/1000.0)
	})
}

func countMiddleware(path string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		metricsMu.Lock()
		requestCounts[path]++
		metricsMu.Unlock()
		next.ServeHTTP(w, r)
	})
}

// --- Handlers ---
func handleAdd(w http.ResponseWriter, r *http.Request) {
	a, b, ok := validateParams(w, r)
	if !ok { return }
	writeResult(w, a+b)
}

func handleSubtract(w http.ResponseWriter, r *http.Request) {
	a, b, ok := validateParams(w, r)
	if !ok { return }
	writeResult(w, a-b)
}

func handleMultiply(w http.ResponseWriter, r *http.Request) {
	a, b, ok := validateParams(w, r)
	if !ok { return }
	writeResult(w, a*b)
}

func handleDivide(w http.ResponseWriter, r *http.Request) {
	a, b, ok := validateParams(w, r)
	if !ok { return }
	if b == 0 {
		writeErrorMsg(w, "division by zero")
		return
	}
	writeResult(w, a/b)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status": "ok"}`))
}

func handleMetrics(w http.ResponseWriter, r *http.Request) {
	metricsMu.Lock()
	counts := make(map[string]int, len(requestCounts))
	for k, v := range requestCounts {
		counts[k] = v
	}
	metricsMu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]map[string]int{"endpoints": counts})
}

// --- Helpers (agent-3) ---
func validateParams(w http.ResponseWriter, r *http.Request) (int, int, bool) {
	aStr := r.URL.Query().Get("a")
	bStr := r.URL.Query().Get("b")
	if aStr == "" {
		writeErrorMsg(w, "missing parameter 'a'")
		return 0, 0, false
	}
	if bStr == "" {
		writeErrorMsg(w, "missing parameter 'b'")
		return 0, 0, false
	}
	a, err := strconv.Atoi(aStr)
	if err != nil {
		writeErrorMsg(w, fmt.Sprintf("invalid integer: '%s'", aStr))
		return 0, 0, false
	}
	b, err := strconv.Atoi(bStr)
	if err != nil {
		writeErrorMsg(w, fmt.Sprintf("invalid integer: '%s'", bStr))
		return 0, 0, false
	}
	return a, b, true
}

func writeResult(w http.ResponseWriter, value int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]int{"result": value})
}

func writeErrorMsg(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// --- main ---
func main() {
	mux := http.NewServeMux()
	mux.Handle("/add", countMiddleware("/add", http.HandlerFunc(handleAdd)))
	mux.Handle("/subtract", countMiddleware("/subtract", http.HandlerFunc(handleSubtract)))
	mux.Handle("/multiply", countMiddleware("/multiply", http.HandlerFunc(handleMultiply)))
	mux.Handle("/divide", countMiddleware("/divide", http.HandlerFunc(handleDivide)))
	mux.HandleFunc("/health", handleHealth)
	mux.HandleFunc("/metrics", handleMetrics)
	handler := loggingMiddleware(mux)
	log.Println("server starting on :8090")
	http.ListenAndServe(":8090", handler)
}
GOEOF

$WS run final -- go build -o /dev/null main.go
FINAL_LAYER=$($WS keep final --message="consolidated: all 6 agents merged" | grep "kept layer" | awk '{print $3}')
echo "  ✓ Consolidated layer: $FINAL_LAYER"
echo ""

# ── 5. Show graph ────────────────────────────────────────────────────────────
echo "▶ Dependency graph:"
echo ""
$WS graph
echo ""

# ── 6. Smoke test ─────────────────────────────────────────────────────────────
echo "▶ Smoke test — building and testing all endpoints..."
$WS run final -- go build -o /tmp/ws-demo-server main.go
/tmp/ws-demo-server &
SERVER_PID=$!
sleep 1

test_ep() {
	local desc="$1" url="$2" expected="$3"
	local actual
	actual=$(curl -s "$url")
	if echo "$actual" | grep -qF "$expected"; then
		echo "  ✓ $desc → $actual"
	else
		echo "  ✗ $desc → got: $actual (expected: $expected)"
		kill $SERVER_PID 2>/dev/null
		exit 1
	fi
}

test_ep "add" "http://localhost:$PORT/add?a=5&b=3" '"result":8'
test_ep "subtract" "http://localhost:$PORT/subtract?a=10&b=4" '"result":6'
test_ep "multiply" "http://localhost:$PORT/multiply?a=6&b=7" '"result":42'
test_ep "divide" "http://localhost:$PORT/divide?a=20&b=4" '"result":5'
test_ep "divide by zero" "http://localhost:$PORT/divide?a=20&b=0" 'division by zero'
test_ep "missing params" "http://localhost:$PORT/add" 'missing parameter'
test_ep "invalid int" "http://localhost:$PORT/add?a=foo&b=bar" 'invalid integer'
test_ep "health" "http://localhost:$PORT/health" 'ok'
test_ep "metrics" "http://localhost:$PORT/metrics" 'endpoints'

kill $SERVER_PID 2>/dev/null
wait $SERVER_PID 2>/dev/null
echo ""

# ── 7. Cleanup ────────────────────────────────────────────────────────────────
echo "▶ Cleanup..."
for ws_name in agent-1 agent-2 agent-3 agent-4 agent-5 agent-6 final; do
	$WS drop "$ws_name" 2>/dev/null || true
done
$WS layer gc
echo "  ✓ Done"
echo ""

echo "╔══════════════════════════════════════════════════════════╗"
echo "║  ✓ Demo complete — 7 agents, 9 layers, 1 binary, 8/8 ✓   ║"
echo "╚══════════════════════════════════════════════════════════╝"
