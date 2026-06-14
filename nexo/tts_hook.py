#!/usr/bin/env python3
"""
Nexo TTS Hook for OpenCode
Integrates TTS into the AI response pipeline
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
HOOK_CONFIG = "/root/primerModelo/nexo/tts_hook_config.json"


class NexoTTS:
    def __init__(self):
        self.output_dir = OUTPUT_DIR
        self.config = self.load_config()
        os.makedirs(self.output_dir, exist_ok=True)
    
    def load_config(self):
        """Load hook configuration"""
        default_config = {
            "enabled": True,
            "auto_tts": True,
            "default_lang": "es",
            "max_length": 2000,
            "clean_text": True,
            "save_history": True
        }
        
        if os.path.exists(HOOK_CONFIG):
            try:
                with open(HOOK_CONFIG, 'r') as f:
                    config = json.load(f)
                    return {**default_config, **config}
            except:
                pass
        
        return default_config
    
    def save_config(self, config):
        """Save hook configuration"""
        with open(HOOK_CONFIG, 'w') as f:
            json.dump(config, f, indent=2)
    
    def clean_for_tts(self, text):
        """Clean text for better TTS output"""
        import re
        
        if not self.config.get("clean_text", True):
            return text
        
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
        
        # Limit length
        max_len = self.config.get("max_length", 2000)
        if len(text) > max_len:
            text = text[:max_len] + "..."
        
        return text
    
    def detect_language(self, text):
        """Simple language detection"""
        text_lower = text.lower()
        
        # Spanish indicators
        spanish_words = ['hola', 'soy', 'como', 'esta', 'que', 'para', 'por', 'con', 'una', 'uno', 'el', 'la', 'los', 'las', 'gracias', 'bien']
        spanish_count = sum(1 for word in spanish_words if word in text_lower)
        
        # English indicators
        english_words = ['hello', 'i', 'am', 'how', 'are', 'you', 'what', 'for', 'with', 'a', 'an', 'the', 'is', 'are', 'thanks', 'good']
        english_count = sum(1 for word in english_words if word in text_lower)
        
        if spanish_count > english_count:
            return "es"
        elif english_count > spanish_count:
            return "en"
        
        return self.config.get("default_lang", "es")
    
    def speak(self, text, lang=None):
        """Convert text to speech and return audio path"""
        if not self.config.get("enabled", True):
            return None
        
        if not text or not text.strip():
            return None
        
        # Detect language if not provided
        if lang is None:
            lang = self.detect_language(text)
        
        # Clean text
        clean_text = self.clean_for_tts(text)
        
        if not clean_text.strip():
            return None
        
        # Generate filename
        text_hash = hashlib.md5(clean_text.encode()).hexdigest()[:8]
        filename = f"response_{text_hash}.mp3"
        filepath = os.path.join(self.output_dir, filename)
        
        # Generate speech
        try:
            tts = gTTS(text=clean_text, lang=lang)
            tts.save(filepath)
            return filepath
        except Exception as e:
            print(f"TTS Error: {e}", file=sys.stderr)
            return None
    
    def speak_and_get_url(self, text, lang=None):
        """Generate speech and return HTTP URL"""
        filepath = self.speak(text, lang)
        if filepath:
            filename = os.path.basename(filepath)
            return f"http://localhost:8765/audio/{filename}"
        return None


# Global instance
tts = NexoTTS()


def auto_tts_hook(response_text):
    """
    Hook function to be called after AI response
    Generates TTS for the response text
    """
    if not tts.config.get("auto_tts", True):
        return None
    
    filepath = tts.speak(response_text)
    if filepath:
        return {
            "audio_file": filepath,
            "audio_url": f"http://localhost:8765/audio/{os.path.basename(filepath)}",
            "text_preview": response_text[:100] + "..."
        }
    return None


if __name__ == "__main__":
    # Test the hook
    import argparse
    
    parser = argparse.ArgumentParser(description="Nexo TTS Hook")
    parser.add_argument("text", nargs="?", help="Text to convert")
    parser.add_argument("--lang", help="Language code")
    parser.add_argument("--config", action="store_true", help="Show config")
    parser.add_argument("--enable", action="store_true", help="Enable auto TTS")
    parser.add_argument("--disable", action="store_true", help="Disable auto TTS")
    
    args = parser.parse_args()
    
    if args.config:
        print(json.dumps(tts.config, indent=2))
    elif args.enable:
        tts.config["auto_tts"] = True
        tts.save_config(tts.config)
        print("✅ Auto TTS enabled")
    elif args.disable:
        tts.config["auto_tts"] = False
        tts.save_config(tts.config)
        print("❌ Auto TTS disabled")
    elif args.text:
        result = auto_tts_hook(args.text)
        if result:
            print(f"🔊 Audio: {result['audio_file']}")
            print(f"🌐 URL: {result['audio_url']}")
        else:
            print("No audio generated")
    else:
        parser.print_help()
