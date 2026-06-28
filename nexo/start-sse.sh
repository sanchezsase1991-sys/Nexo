#!/bin/bash
# ============================================================
#  NEXO MCP SSE Server - Persistent Service
# ============================================================

NEXO_BIN="/root/primerModelo/nexo/cortex"
ADDR=":8765"
PIDFILE="/tmp/nexo-mcp-sse.pid"

# Check if already running
if [ -f "$PIDFILE" ]; then
    OLD_PID=$(cat "$PIDFILE")
    if kill -0 "$OLD_PID" 2>/dev/null; then
        echo "Nexo MCP SSE server already running (PID $OLD_PID)"
        exit 0
    fi
    rm -f "$PIDFILE"
fi

# Start server
echo "Starting Nexo MCP SSE server on $ADDR..."
nohup "$NEXO_BIN" --sse "$ADDR" > /tmp/nexo-mcp-sse.log 2>&1 &
echo $! > "$PIDFILE"
sleep 1

# Verify
if kill -0 "$(cat $PIDFILE)" 2>/dev/null; then
    echo "✓ Nexo MCP SSE server started (PID $(cat $PIDFILE))"
    echo "  SSE: http://localhost${ADDR}/sse"
    echo "  MSG: http://localhost${ADDR}/message"
    echo "  Health: http://localhost${ADDR}/health"
else
    echo "✗ Failed to start server"
    cat /tmp/nexo-mcp-sse.log
    exit 1
fi
