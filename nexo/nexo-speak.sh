#!/bin/bash
# Nexo TTS Auto Response Wrapper
# This script generates audio for AI responses

# Get the response text from arguments or stdin
if [ $# -gt 0 ]; then
    TEXT="$*"
else
    TEXT=$(cat)
fi

# Generate audio
RESULT=$(echo "$TEXT" | python3 /root/primerModelo/nexo/response_tts.py)

# Check if successful
if echo "$RESULT" | grep -q '"success": true'; then
    AUDIO_URL=$(echo "$RESULT" | python3 -c "import sys, json; print(json.load(sys.stdin)['audio_url'])")
    AUDIO_FILE=$(echo "$RESULT" | python3 -c "import sys, json; print(json.load(sys.stdin)['audio_file'])")
    echo "🔊 Audio generado: $AUDIO_FILE"
    echo "🌐 URL: $AUDIO_URL"
else
    echo "❌ Error generando audio"
    exit 1
fi
