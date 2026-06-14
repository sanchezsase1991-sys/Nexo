#!/usr/bin/env python3
"""
Nexo Auto Response with TTS
Wrapper that generates audio for AI responses
"""

import os
import sys
import json
import hashlib
from pathlib import Path

# Add parent directory to path
sys.path.insert(0, str(Path(__file__).parent))

from gtts import gTTS

OUTPUT_DIR = "/tmp/nexo-tts"


def generate_response_audio(text, lang=None):
    """Generate audio for a response and return the file path"""
    os.makedirs(OUTPUT_DIR, exist_ok=True)
    
    # Simple language detection
    if lang is None:
        text_lower = text.lower()
        spanish_words = ['hola', 'soy', 'como', 'esta', 'que', 'para', 'por', 'con', 'gracias']
        spanish_count = sum(1 for word in spanish_words if word in text_lower)
        english_words = ['hello', 'i', 'am', 'how', 'are', 'you', 'what', 'thanks']
        english_count = sum(1 for word in english_words if word in text_lower)
        
        if spanish_count > english_count:
            lang = "es"
        else:
            lang = "en"
    
    # Clean text for TTS
    import re
    text = re.sub(r'```.*?```', '', text, flags=re.DOTALL)
    text = re.sub(r'`[^`]*`', '', text)
    text = re.sub(r'\*\*([^*]*)\*\*', r'\1', text)
    text = re.sub(r'\*([^*]*)\*', r'\1', text)
    text = re.sub(r'\[([^\]]*)\]\([^)]*\)', r'\1', text)
    text = re.sub(r'[|}{~><]', '', text)
    text = re.sub(r'\s+', ' ', text).strip()
    
    # Limit length
    if len(text) > 2000:
        text = text[:2000] + "..."
    
    if not text:
        return None
    
    # Generate filename
    text_hash = hashlib.md5(text.encode()).hexdigest()[:8]
    filename = f"response_{text_hash}.mp3"
    filepath = os.path.join(OUTPUT_DIR, filename)
    
    # Generate speech
    try:
        tts = gTTS(text=text, lang=lang)
        tts.save(filepath)
        return filepath
    except Exception as e:
        print(f"Error: {e}", file=sys.stderr)
        return None


if __name__ == "__main__":
    # Read response from stdin or arguments
    if len(sys.argv) > 1:
        text = " ".join(sys.argv[1:])
    else:
        text = sys.stdin.read()
    
    if not text.strip():
        print("No text provided", file=sys.stderr)
        sys.exit(1)
    
    # Generate audio
    filepath = generate_response_audio(text)
    
    if filepath:
        filename = os.path.basename(filepath)
        print(json.dumps({
            "success": True,
            "audio_file": filepath,
            "audio_url": f"http://localhost:8765/audio/{filename}",
            "text_preview": text[:100] + "..."
        }))
    else:
        print(json.dumps({"success": False, "error": "Failed to generate audio"}))
        sys.exit(1)
