#!/bin/bash
# ============================================================
#  NEXO MCP PERSISTENT - Creates a FIFO-based MCP wrapper
#  This keeps the server alive between OpenCode tool calls
# ============================================================

NEXO_BIN="/root/primerModelo/nexo/cortex"
FIFO="/tmp/nexo-mcp-fifo"
LOG="/tmp/nexo-mcp-persistent.log"

# Clean up old FIFO
rm -f "$FIFO"

# Create FIFO
mkfifo "$FIFO"
echo "📡 FIFO created: $FIFO" >> "$LOG"

# Function to cleanup on exit
cleanup() {
    rm -f "$FIFO"
    kill $SERVER_PID 2>/dev/null
    echo "🛑 Cleaned up" >> "$LOG"
}
trap cleanup EXIT INT TERM

# Start the actual MCP server, reading from FIFO
# The key: FIFO stays open even when no writer is connected
$NEXO_BIN --mcp < "$FIFO" > /dev/null 2>>"$LOG" &
SERVER_PID=$!
echo "🧠 MCP server started (PID $SERVER_PID)" >> "$LOG"

# Keep the FIFO writer end open permanently
exec 3>"$FIFO"
echo "📡 FIFO writer open" >> "$LOG"

# Now forward any input from our stdin to the FIFO
# This is what OpenCode sends to us
while IFS= read -r line; do
    echo "$line" >&3
done

# If we get here, stdin was closed
echo "📡 stdin closed, keeping server alive..." >> "$LOG"
wait $SERVER_PID 2>/dev/null
