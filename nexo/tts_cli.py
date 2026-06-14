#!/usr/bin/env python3
"""
Nexo TTS - Simple command line interface
"""

import argparse
import os
import hashlib
from gtts import gTTS

OUTPUT_DIR = "/tmp/nexo-tts"

LANGUAGES = {
    "es": "Spanish",
    "en": "English",
    "fr": "French",
    "de": "German",
    "it": "Italian",
    "pt": "Portuguese",
    "ja": "Japanese",
    "ko": "Korean",
    "zh": "Chinese",
}

def speak(text, lang="es", output=None):
    if not output:
        os.makedirs(OUTPUT_DIR, exist_ok=True)
        text_hash = hashlib.md5(text.encode()).hexdigest()[:8]
        output = os.path.join(OUTPUT_DIR, f"speech_{text_hash}.mp3")
    
    tts = gTTS(text=text, lang=lang)
    tts.save(output)
    print(f"✅ Audio: {output}")
    print(f"   Idioma: {LANGUAGES.get(lang, lang)}")
    print(f"   Texto: {len(text)} caracteres")
    return output

def list_languages():
    print("Idiomas disponibles:")
    for k, v in LANGUAGES.items():
        print(f"  {k}: {v}")

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Nexo TTS - Text to Speech")
    parser.add_argument("text", nargs="?", help="Texto a convertir")
    parser.add_argument("-l", "--lang", default="es", help="Idioma (default: es)")
    parser.add_argument("-o", "--output", help="Archivo de salida")
    parser.add_argument("--list-languages", action="store_true", help="Listar idiomas")
    
    args = parser.parse_args()
    
    if args.list_languages:
        list_languages()
    elif args.text:
        speak(args.text, args.lang, args.output)
    else:
        parser.print_help()
