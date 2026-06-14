#!/usr/bin/env python3
"""
Nexo TTS Web Player
Starts a web server and opens the player in browser
"""

import os
import sys
import webbrowser
import threading
from http.server import HTTPServer, SimpleHTTPRequestHandler
from urllib.parse import urlparse, parse_qs
import json
import hashlib

from gtts import gTTS

OUTPUT_DIR = "/tmp/nexo-tts"
PORT = 8765

# Supported languages
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


class TTSHandler(SimpleHTTPRequestHandler):
    def do_GET(self):
        parsed = urlparse(self.path)
        
        # Serve audio files
        if parsed.path.startswith("/audio/"):
            filename = parsed.path[7:]  # Remove /audio/
            filepath = os.path.join(OUTPUT_DIR, filename)
            if os.path.exists(filepath):
                self.send_response(200)
                self.send_header("Content-Type", "audio/mpeg")
                self.send_header("Access-Control-Allow-Origin", "*")
                self.end_headers()
                with open(filepath, "rb") as f:
                    self.wfile.write(f.read())
            else:
                self.send_error(404, "Audio file not found")
            return
        
        # TTS API endpoint
        if parsed.path == "/tts":
            params = parse_qs(parsed.query)
            text = params.get("text", [""])[0]
            lang = params.get("lang", ["es"])[0]
            
            if not text:
                self.send_json({"error": "No text provided"}, 400)
                return
            
            # Generate audio
            os.makedirs(OUTPUT_DIR, exist_ok=True)
            text_hash = hashlib.md5(text.encode()).hexdigest()[:8]
            filename = f"speech_{text_hash}.mp3"
            filepath = os.path.join(OUTPUT_DIR, filename)
            
            try:
                tts = gTTS(text=text, lang=lang)
                tts.save(filepath)
                self.send_json({
                    "success": True,
                    "audio_url": f"/audio/{filename}",
                    "text": text,
                    "lang": lang,
                    "lang_name": LANGUAGES.get(lang, lang),
                })
            except Exception as e:
                self.send_json({"error": str(e)}, 500)
            return
        
        # List languages
        if parsed.path == "/languages":
            self.send_json({"languages": LANGUAGES})
            return
        
        # List audio files
        if parsed.path == "/list":
            files = []
            if os.path.exists(OUTPUT_DIR):
                for f in os.listdir(OUTPUT_DIR):
                    if f.endswith(".mp3"):
                        filepath = os.path.join(OUTPUT_DIR, f)
                        files.append({
                            "filename": f,
                            "url": f"/audio/{f}",
                            "size": os.path.getsize(filepath),
                        })
            self.send_json({"files": files})
            return
        
        # Default: serve HTML player
        self.send_response(200)
        self.send_header("Content-Type", "text/html")
        self.end_headers()
        
        # Read the HTML file
        html_path = os.path.join(os.path.dirname(__file__), "tts_player.html")
        if os.path.exists(html_path):
            with open(html_path, "rb") as f:
                self.wfile.write(f.read())
        else:
            self.wfile.write(b"<h1>Nexo TTS Player</h1><p>HTML file not found</p>")
    
    def send_json(self, data, status=200):
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Access-Control-Allow-Origin", "*")
        self.end_headers()
        self.wfile.write(json.dumps(data).encode())


def open_browser():
    """Open browser after a short delay"""
    import time
    time.sleep(1)
    webbrowser.open(f"http://localhost:{PORT}")


def main():
    os.makedirs(OUTPUT_DIR, exist_ok=True)
    
    print(f"🔊 Nexo TTS Server starting...")
    print(f"📁 Audio files: {OUTPUT_DIR}")
    print(f"🌐 Server: http://localhost:{PORT}")
    
    # Open browser in a separate thread
    threading.Thread(target=open_browser, daemon=True).start()
    
    server = HTTPServer(("0.0.0.0", PORT), TTSHandler)
    print(f"\n✅ Server running at http://localhost:{PORT}")
    print("🌐 Opening browser...")
    print("\nPress Ctrl+C to stop")
    
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        print("\n👋 Server stopped")
        server.server_close()


if __name__ == "__main__":
    main()
