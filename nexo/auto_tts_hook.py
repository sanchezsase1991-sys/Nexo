#!/usr/bin/env python3
"""
Nexo Auto-TTS Integration for OpenCode
Automatically generates audio for AI responses
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


class NexoAutoTTS:
    def __init__(self):
        self.output_dir = OUTPUT_DIR
        os.makedirs(self.output_dir, exist_ok=True)
    
    def clean_text(self, text):
        """Clean text for TTS"""
        import re
        
        # Remove markdown
        text = re.sub(r'```.*?```', '', text, flags=re.DOTALL)
        text = re.sub(r'`[^`]*`', '', text)
        text = re.sub(r'\*\*([^*]*)\*\*', r'\1', text)
        text = re.sub(r'\*([^*]*)\*', r'\1', text)
        text = re.sub(r'\[([^\]]*)\]\([^)]*\)', r'\1', text)
        
        # Remove emojis
        emoji_pattern = re.compile("["
            u"\U0001F600-\U0001F64F"
            u"\U0001F300-\U0001F5FF"
            u"\U0001F680-\U0001F6FF"
            u"\U0001F1E0-\U0001F1FF"
            u"\U00002702-\U000027B0"
            u"\U000024C2-\U0001F251"
            "]+", flags=re.UNICODE)
        text = emoji_pattern.sub(r'', text)
        
        # Remove special characters
        text = re.sub(r'[|}{~><]', '', text)
        
        # Clean whitespace
        text = re.sub(r'\s+', ' ', text).strip()
        
        # Limit length
        if len(text) > 2000:
            text = text[:2000] + "..."
        
        return text
    
    def detect_language(self, text):
        """Simple language detection"""
        text_lower = text.lower()
        
        spanish_words = ['hola', 'soy', 'como', 'esta', 'que', 'para', 'por', 'con', 'gracias', 'bien']
        spanish_count = sum(1 for word in spanish_words if word in text_lower)
        
        english_words = ['hello', 'i', 'am', 'how', 'are', 'you', 'what', 'thanks', 'good']
        english_count = sum(1 for word in english_words if word in text_lower)
        
        if spanish_count > english_count:
            return "es"
        return "en"
    
    def generate_audio(self, text, lang=None):
        """Generate audio for text"""
        if not text or not text.strip():
            return None
        
        # Clean text
        clean_text = self.clean_text(text)
        
        if not clean_text:
            return None
        
        # Detect language
        if lang is None:
            lang = self.detect_language(clean_text)
        
        # Generate filename
        text_hash = hashlib.md5(clean_text.encode()).hexdigest()[:8]
        filename = f"response_{text_hash}.mp3"
        filepath = os.path.join(self.output_dir, filename)
        
        # Generate speech
        try:
            tts = gTTS(text=clean_text, lang=lang)
            tts.save(filepath)
            return {
                "file": filepath,
                "url": f"http://localhost:8765/audio/{filename}",
                "lang": lang,
                "text_preview": clean_text[:100] + "..."
            }
        except Exception as e:
            print(f"TTS Error: {e}", file=sys.stderr)
            return None


# Global instance
auto_tts = NexoAutoTTS()


def hook(response_text):
    """
    Hook function to be called after AI response
    Returns audio info if successful
    """
    return auto_tts.generate_audio(response_text)


if __name__ == "__main__":
    # Test the hook
    if len(sys.argv) > 1:
        text = " ".join(sys.argv[1:])
    else:
        text = sys.stdin.read()
    
    result = hook(text)
    
    if result:
        print(json.dumps(result, indent=2))
    else:
        print("No audio generated")
        sys.exit(1)
