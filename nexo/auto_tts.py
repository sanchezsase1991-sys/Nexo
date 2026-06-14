#!/usr/bin/env python3
"""
Nexo Auto-TTS Hook
Automatically converts AI responses to speech
"""

import os
import sys
import hashlib
import json
from gtts import gTTS

OUTPUT_DIR = "/tmp/nexo-tts"
HISTORY_FILE = "/tmp/nexo-tts/history.json"

# Supported languages
DEFAULT_LANG = "es"


def text_to_speech(text, lang=DEFAULT_LANG, auto_play=False):
    """Convert text to speech and return audio path"""
    os.makedirs(OUTPUT_DIR, exist_ok=True)
    
    # Clean text for TTS (remove markdown, emojis, etc.)
    clean_text = clean_for_tts(text)
    
    if not clean_text.strip():
        return None
    
    # Generate filename
    text_hash = hashlib.md5(clean_text.encode()).hexdigest()[:8]
    filename = f"response_{text_hash}.mp3"
    filepath = os.path.join(OUTPUT_DIR, filename)
    
    # Generate speech
    try:
        tts = gTTS(text=clean_text, lang=lang)
        tts.save(filepath)
        
        # Save to history
        save_to_history(clean_text, lang, filepath)
        
        return filepath
    except Exception as e:
        print(f"Error generating TTS: {e}", file=sys.stderr)
        return None


def clean_for_tts(text):
    """Clean text for better TTS output"""
    import re
    
    # Remove markdown formatting
    text = re.sub(r'```.*?```', '', text, flags=re.DOTALL)  # Code blocks
    text = re.sub(r'`[^`]*`', '', text)  # Inline code
    text = re.sub(r'\*\*([^*]*)\*\*', r'\1', text)  # Bold
    text = re.sub(r'\*([^*]*)\*', r'\1', text)  # Italic
    text = re.sub(r'\[([^\]]*)\]\([^)]*\)', r'\1', text)  # Links
    
    # Remove emojis (basic)
    emoji_pattern = re.compile("["
        u"\U0001F600-\U0001F64F"
        u"\U0001F300-\U0001F5FF"
        u"\U0001F680-\U0001F6FF"
        u"\U0001F1E0-\U0001F1FF"
        u"\U00002702-\U000027B0"
        u"\U000024C2-\U0001F251"
        "]+", flags=re.UNICODE)
    text = emoji_pattern.sub(r'', text)
    
    # Remove special characters but keep basic punctuation
    text = re.sub(r'[|}{~><]', '', text)
    
    # Clean up whitespace
    text = re.sub(r'\s+', ' ', text).strip()
    
    # Limit length for TTS (Google TTS has limits)
    if len(text) > 5000:
        text = text[:5000] + "..."
    
    return text


def save_to_history(text, lang, filepath):
    """Save TTS generation to history"""
    history = []
    if os.path.exists(HISTORY_FILE):
        try:
            with open(HISTORY_FILE, 'r') as f:
                history = json.load(f)
        except:
            history = []
    
    history.insert(0, {
        "text": text[:200],
        "lang": lang,
        "filepath": filepath,
        "timestamp": os.path.getmtime(filepath)
    })
    
    # Keep only last 50 entries
    history = history[:50]
    
    with open(HISTORY_FILE, 'w') as f:
        json.dump(history, f, indent=2)


def detect_language(text):
    """Simple language detection based on common words"""
    text_lower = text.lower()
    
    # Spanish indicators
    spanish_words = ['hola', 'soy', 'como', 'esta', 'que', 'para', 'por', 'con', 'una', 'uno', 'el', 'la', 'los', 'las']
    spanish_count = sum(1 for word in spanish_words if word in text_lower)
    
    # English indicators
    english_words = ['hello', 'i', 'am', 'how', 'are', 'you', 'what', 'for', 'with', 'a', 'an', 'the', 'is', 'are']
    english_count = sum(1 for word in english_words if word in text_lower)
    
    if spanish_count > english_count:
        return "es"
    elif english_count > spanish_count:
        return "en"
    return DEFAULT_LANG


if __name__ == "__main__":
    # Read text from stdin or arguments
    if len(sys.argv) > 1:
        text = " ".join(sys.argv[1:])
    else:
        text = sys.stdin.read()
    
    if not text.strip():
        print("No text provided", file=sys.stderr)
        sys.exit(1)
    
    # Detect language
    lang = detect_language(text)
    
    # Generate speech
    filepath = text_to_speech(text, lang)
    
    if filepath:
        print(f"🔊 Audio: {filepath}")
        print(f"📝 Texto: {text[:100]}...")
        print(f"🌍 Idioma: {lang}")
    else:
        print("Error generating audio", file=sys.stderr)
        sys.exit(1)
