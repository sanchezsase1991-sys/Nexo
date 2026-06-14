#!/usr/bin/env python3
"""
Nexo TTS MCP Server
Simple text-to-speech server using gTTS
"""

import asyncio
import json
import os
import tempfile
from pathlib import Path

from gtts import gTTS
from mcp import types
from mcp.server import Server
from mcp.server.stdio import stdio_server


app = Server("nexo-tts")

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

# Default output directory
OUTPUT_DIR = os.environ.get("TTS_OUTPUT_DIR", "/tmp/nexo-tts")


@app.list_tools()
async def list_tools():
    return [
        types.Tool(
            name="speak",
            description="Convert text to speech and save as MP3",
            inputSchema={
                "type": "object",
                "properties": {
                    "text": {
                        "type": "string",
                        "description": "Text to convert to speech",
                    },
                    "lang": {
                        "type": "string",
                        "description": "Language code (es, en, fr, de, it, pt, ja, ko, zh)",
                        "default": "es",
                    },
                    "output_path": {
                        "type": "string",
                        "description": "Output file path (optional, auto-generated if not provided)",
                    },
                },
                "required": ["text"],
            },
        ),
        types.Tool(
            name="list_languages",
            description="List available languages for TTS",
            inputSchema={
                "type": "object",
                "properties": {},
            },
        ),
    ]


@app.call_tool()
async def call_tool(name: str, arguments: dict):
    if name == "speak":
        text = arguments.get("text", "")
        lang = arguments.get("lang", "es")
        output_path = arguments.get("output_path")

        if not text:
            return [types.TextContent(type="text", text="Error: No text provided")]

        # Generate output path if not provided
        if not output_path:
            os.makedirs(OUTPUT_DIR, exist_ok=True)
            import hashlib
            text_hash = hashlib.md5(text.encode()).hexdigest()[:8]
            output_path = os.path.join(OUTPUT_DIR, f"speech_{text_hash}.mp3")

        # Generate speech
        try:
            tts = gTTS(text=text, lang=lang)
            tts.save(output_path)
            return [
                types.TextContent(
                    type="text",
                    text=f"✅ Audio saved to: {output_path}\nLanguage: {LANGUAGES.get(lang, lang)}\nText length: {len(text)} chars",
                )
            ]
        except Exception as e:
            return [types.TextContent(type="text", text=f"Error generating speech: {str(e)}")]

    elif name == "list_languages":
        lang_list = "\n".join([f"  {k}: {v}" for k, v in LANGUAGES.items()])
        return [types.TextContent(type="text", text=f"Available languages:\n{lang_list}")]

    return [types.TextContent(type="text", text=f"Unknown tool: {name}")]


async def main():
    async with stdio_server() as (read_stream, write_stream):
        await app.run(read_stream, write_stream, app.create_initialization_options())


if __name__ == "__main__":
    asyncio.run(main())
