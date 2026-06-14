#!/usr/bin/env python3
"""
Nexo TTS - Generate and play audio
"""

import os
import hashlib
from gtts import gTTS

OUTPUT_DIR = "/tmp/nexo-tts"

def speak(text, lang="es", auto_play=True):
    """Generate speech and optionally play it"""
    os.makedirs(OUTPUT_DIR, exist_ok=True)
    
    # Generate filename
    text_hash = hashlib.md5(text.encode()).hexdigest()[:8]
    filename = f"speech_{text_hash}.mp3"
    filepath = os.path.join(OUTPUT_DIR, filename)
    
    # Generate speech
    tts = gTTS(text=text, lang=lang)
    tts.save(filepath)
    
    print(f"✅ Audio generado: {filepath}")
    
    return filepath

if __name__ == "__main__":
    import sys
    
    text = " ".join(sys.argv[1:]) if len(sys.argv) > 1 else "Hola S, soy Nexo"
    lang = "es"
    
    if "--lang" in sys.argv:
        idx = sys.argv.index("--lang")
        if idx + 1 < len(sys.argv):
            lang = sys.argv[idx + 1]
    
    filepath = speak(text, lang, auto_play=False)
    print(f"📁 Archivo: {filepath}")
    print(f"🌐 Para escuchar, abre el archivo en tu navegador o reproductor")
    print(f"   O ejecuta: python3 tts_player.py")
