#!/bin/bash
# ─── Tunnel Load Test ───────────────────────────────────────────────────────────
# Tests randomness, stability, and retry functionality of the CONNECT proxy
# Usage: ./tunnel-load-test.sh [duration_minutes] [concurrency]
# ─────────────────────────────────────────────────────────────────────────────────

PROXY="http://gateway.iploop.io:8880"
DURATION_MIN=${1:-60}
CONCURRENCY=${2:-5}
DURATION_SEC=$((DURATION_MIN * 60))
END_TIME=$(($(date +%s) + DURATION_SEC))

LOG_DIR="/root/clawd-secure/iploop-platform/services/ws-stability-test/tests/results"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
LOG_FILE="$LOG_DIR/tunnel-test-$TIMESTAMP.log"
SUMMARY_FILE="$LOG_DIR/tunnel-test-$TIMESTAMP-summary.log"
CSV_FILE="$LOG_DIR/tunnel-test-$TIMESTAMP.csv"
mkdir -p "$LOG_DIR"

# ─── Target Sites ───────────────────────────────────────────────────────────────
SITES=(
    "https://www.bbc.com"
    "https://www.cnn.com"
    "https://www.reuters.com"
    "https://www.nytimes.com"
    "https://www.theguardian.com"
    "https://news.ycombinator.com"
    "https://www.washingtonpost.com"
    "https://www.aljazeera.com"
    "https://www.bloomberg.com"
    "https://techcrunch.com"
    "https://www.wired.com"
    "https://arstechnica.com"
    "https://www.nbcnews.com"
    "https://www.foxnews.com"
    "https://www.espn.com"
    "https://www.wikipedia.org"
    "https://www.amazon.com"
    "https://www.reddit.com"
    "https://www.stackoverflow.com"
    "https://www.github.com"
    "https://httpbin.org/ip"
    "https://ip2location.io/ip"
    "https://ifconfig.me"
    "https://api.ipify.org"
    "https://ipinfo.io/ip"
)

# Counters (atomic via temp files)
STATS_DIR=$(mktemp -d)
echo "0" > "$STATS_DIR/total"
echo "0" > "$STATS_DIR/success"
echo "0" > "$STATS_DIR/fail"
echo "0" > "$STATS_DIR/timeout"
echo "0" > "$STATS_DIR/bytes"
touch "$STATS_DIR/ips"
touch "$STATS_DIR/latencies"

# CSV header
echo "timestamp,site,status_code,time_total,time_connect,time_starttfb,size_download,exit_ip,result" > "$CSV_FILE"

log() {
    echo "[$(date '+%H:%M:%S')] $*" | tee -a "$LOG_FILE"
}

increment() {
    flock "$STATS_DIR/$1.lock" bash -c "echo \$((\$(cat $STATS_DIR/$1) + 1)) > $STATS_DIR/$1"
}

add_bytes() {
    flock "$STATS_DIR/bytes.lock" bash -c "echo \$((\$(cat $STATS_DIR/bytes) + $1)) > $STATS_DIR/bytes"
}

# ─── Single Request Worker ──────────────────────────────────────────────────────
do_curl_request() {
    local site="${SITES[$((RANDOM % ${#SITES[@]}))]}"
    local ts=$(date '+%Y-%m-%d %H:%M:%S')
    
    local output
    output=$(curl -x "$PROXY" "$site" \
        -k -s -L \
        --max-time 30 \
        --connect-timeout 15 \
        -o /dev/null \
        -w '%{http_code}|%{time_total}|%{time_connect}|%{time_starttfb}|%{size_download}|%{remote_ip}' \
        2>/dev/null)
    
    local exit_code=$?
    
    IFS='|' read -r status_code time_total time_connect time_starttfb size_download remote_ip <<< "$output"
    
    local result="FAIL"
    if [[ $exit_code -eq 0 && "$status_code" =~ ^[23] ]]; then
        result="OK"
        increment success
        add_bytes "${size_download%.*}"
    elif [[ $exit_code -eq 28 ]]; then
        result="TIMEOUT"
        increment timeout
    else
        result="FAIL"
        increment fail
    fi
    increment total
    
    # Track unique IPs (from IP-check sites)
    if [[ "$remote_ip" != "" && "$remote_ip" != "0.0.0.0" ]]; then
        echo "$remote_ip" >> "$STATS_DIR/ips"
    fi
    
    # Track latencies for successful requests
    if [[ "$result" == "OK" ]]; then
        echo "$time_total" >> "$STATS_DIR/latencies"
    fi
    
    # Log
    local site_short=$(echo "$site" | sed 's|https\?://||;s|/.*||')
    echo "$ts,$site_short,$status_code,$time_total,$time_connect,$time_starttfb,$size_download,$remote_ip,$result" >> "$CSV_FILE"
    
    if [[ "$result" != "OK" ]]; then
        log "  ❌ $result $site_short → $status_code (${time_total}s) exit=$exit_code"
    fi
}

# ─── Headless Chrome Worker ─────────────────────────────────────────────────────
do_chrome_request() {
    local site="${SITES[$((RANDOM % ${#SITES[@]}))]}"
    local site_short=$(echo "$site" | sed 's|https\?://||;s|/.*||')
    local ts=$(date '+%Y-%m-%d %H:%M:%S')
    
    local start_time=$(date +%s%N)
    
    local output
    output=$(/snap/bin/chromium --headless=new --no-sandbox --disable-gpu \
        --proxy-server="$PROXY" \
        --ignore-certificate-errors \
        --timeout=30000 \
        --dump-dom "$site" 2>/dev/null | wc -c)
    
    local exit_code=$?
    local end_time=$(date +%s%N)
    local elapsed_ms=$(( (end_time - start_time) / 1000000 ))
    local elapsed_s=$(echo "scale=3; $elapsed_ms / 1000" | bc)
    
    local result="FAIL"
    if [[ $exit_code -eq 0 && "$output" -gt 1000 ]]; then
        result="OK"
        increment success
        add_bytes "$output"
    elif [[ $elapsed_ms -gt 29000 ]]; then
        result="TIMEOUT"
        increment timeout
    else
        increment fail
    fi
    increment total
    
    echo "$ts,$site_short,chrome,$elapsed_s,0,0,$output,,${result}_CHROME" >> "$CSV_FILE"
    
    if [[ "$result" == "OK" ]]; then
        log "  🌐 Chrome OK $site_short → ${output} bytes (${elapsed_s}s)"
    else
        log "  ❌ Chrome $result $site_short (${elapsed_s}s) size=$output"
    fi
}

# ─── Print Stats ────────────────────────────────────────────────────────────────
print_stats() {
    local total=$(cat "$STATS_DIR/total")
    local success=$(cat "$STATS_DIR/success")
    local fail=$(cat "$STATS_DIR/fail")
    local timeout=$(cat "$STATS_DIR/timeout")
    local bytes=$(cat "$STATS_DIR/bytes")
    local unique_ips=$(sort -u "$STATS_DIR/ips" 2>/dev/null | wc -l)
    local elapsed=$(($(date +%s) - START_TIME))
    local remaining=$((END_TIME - $(date +%s)))
    
    # Calculate avg latency
    local avg_latency="N/A"
    if [[ -s "$STATS_DIR/latencies" ]]; then
        avg_latency=$(awk '{ sum += $1; n++ } END { if(n>0) printf "%.2f", sum/n }' "$STATS_DIR/latencies")
    fi
    
    local success_rate=0
    if [[ $total -gt 0 ]]; then
        success_rate=$((success * 100 / total))
    fi
    
    local bytes_mb=$((bytes / 1048576))
    local rps=0
    if [[ $elapsed -gt 0 ]]; then
        rps=$((total / elapsed))
    fi
    
    log "━━━ Stats (${elapsed}s elapsed, ${remaining}s remaining) ━━━"
    log "  Total: $total | ✅ $success | ❌ $fail | ⏱ $timeout | Rate: ${success_rate}%"
    log "  Avg latency: ${avg_latency}s | Throughput: ${rps} req/s | Data: ${bytes_mb}MB"
    log "  Unique IPs: $unique_ips"
    log "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
}

# ─── Main ────────────────────────────────────────────────────────────────────────
START_TIME=$(date +%s)

log "╔══════════════════════════════════════════════════╗"
log "║     Tunnel Load Test — ${DURATION_MIN}min, ${CONCURRENCY} concurrent     ║"
log "║     Proxy: $PROXY                ║"
log "║     Sites: ${#SITES[@]} targets                        ║"
log "╚══════════════════════════════════════════════════╝"
log ""

CURL_WORKERS=$((CONCURRENCY - 1))
CHROME_WORKERS=1
if [[ $CONCURRENCY -le 2 ]]; then
    CURL_WORKERS=$CONCURRENCY
    CHROME_WORKERS=0
fi

log "Workers: $CURL_WORKERS curl + $CHROME_WORKERS chrome"
log ""

# Stats printer (every 60s)
(
    while [[ $(date +%s) -lt $END_TIME ]]; do
        sleep 60
        print_stats
    done
) &
STATS_PID=$!

# Request loop
REQUEST_NUM=0
while [[ $(date +%s) -lt $END_TIME ]]; do
    # Launch concurrent batch
    pids=()
    
    for ((i=0; i<CURL_WORKERS; i++)); do
        do_curl_request &
        pids+=($!)
    done
    
    # Chrome request every 10th batch
    if [[ $CHROME_WORKERS -gt 0 && $((REQUEST_NUM % 10)) -eq 0 ]]; then
        do_chrome_request &
        pids+=($!)
    fi
    
    # Wait for batch
    for pid in "${pids[@]}"; do
        wait "$pid" 2>/dev/null
    done
    
    REQUEST_NUM=$((REQUEST_NUM + 1))
    
    # Small delay between batches to not overwhelm
    sleep 2
done

# Final stats
kill $STATS_PID 2>/dev/null
wait $STATS_PID 2>/dev/null

log ""
log "╔══════════════════════════════════════════════════╗"
log "║              FINAL RESULTS                      ║"
log "╚══════════════════════════════════════════════════╝"
print_stats

# Latency distribution
if [[ -s "$STATS_DIR/latencies" ]]; then
    log ""
    log "Latency distribution:"
    awk '{
        if ($1 < 1) bucket="  < 1s"
        else if ($1 < 2) bucket="  1-2s"
        else if ($1 < 5) bucket="  2-5s"
        else if ($1 < 10) bucket=" 5-10s"
        else if ($1 < 20) bucket="10-20s"
        else bucket="  20s+"
        counts[bucket]++
    } END {
        for (b in counts) print b ": " counts[b]
    }' "$STATS_DIR/latencies" | sort | while read line; do
        log "  $line"
    done
fi

# Top exit IPs
if [[ -s "$STATS_DIR/ips" ]]; then
    log ""
    log "Top 10 exit IPs:"
    sort "$STATS_DIR/ips" | uniq -c | sort -rn | head -10 | while read count ip; do
        log "  $ip: $count requests"
    done
fi

log ""
log "CSV: $CSV_FILE"
log "Log: $LOG_FILE"

# Cleanup
rm -rf "$STATS_DIR"
