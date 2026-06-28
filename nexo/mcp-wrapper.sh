#!/bin/bash
# ============================================================
#  NEXO MCP WRAPPER - Keeps the MCP server alive
#  This wrapper maintains a persistent stdin pipe so the
#  server process doesn't die between tool calls
# ============================================================

NEXO_BIN="/root/primerModelo/nexo/cortex"
FIFO_IN="/tmp/nexo-mcp-fifo-in"
FIFO_OUT="/tmp/nexo-mcp-fifo-out"

# Create FIFOs if they don't exist
mkfifo "$FIFO_IN" 2>/dev/null || true
mkfifo "$FIFO_OUT" 2>/dev/null || true

# Start the actual MCP server with FIFO stdin/stdout
$NEXO_BIN --mcp < "$FIFO_IN" > "$FIFO_OUT" 2>/dev/null &
SERVER_PID=$!

# Keep stdin open and forward to the server
exec 3>"$FIFO_IN"

# Forward all input from real stdin to the FIFO
while IFS= read -r line; do
    echo "$line" >&3
done &
READER_PID=$!

# Read from output FIFO and write to real stdout
while IFS= read -r line; do
    echo "$line"
done < "$FIFO_OUT" &
WRITER_PID=$!

# Wait for server to finish
wait $SERVER_PID 2>/dev/null

# Cleanup
kill $READER_PID $WRITER_PID 2>/dev/null
exec 3>&-
rm -f "$FIFO_IN" "$FIFO_OUT"
