#!/usr/bin/env python3
"""
Nexo TTS MCP Server
Text-to-speech server with Nexo memory integration
"""

import asyncio
import json
import os
import subprocess
import sys
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

# Nexo binary path
NEXO_BIN = os.environ.get("NEXO_BIN", "/usr/local/bin/cortex")


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
        types.Tool(
            name="speak_and_remember",
            description="Convert text to speech and store in Nexo memory",
            inputSchema={
                "type": "object",
                "properties": {
                    "text": {
                        "type": "string",
                        "description": "Text to convert to speech and remember",
                    },
                    "lang": {
                        "type": "string",
                        "description": "Language code",
                        "default": "es",
                    },
                    "title": {
                        "type": "string",
                        "description": "Title for the memory episode",
                    },
                    "importance": {
                        "type": "number",
                        "description": "Importance level (0.0-1.0)",
                        "default": 0.5,
                    },
                },
                "required": ["text"],
            },
        ),
        types.Tool(
            name="recall_and_speak",
            description="Recall memory context and speak it aloud",
            inputSchema={
                "type": "object",
                "properties": {
                    "query": {
                        "type": "string",
                        "description": "Query to recall from memory",
                    },
                    "lang": {
                        "type": "string",
                        "description": "Language code",
                        "default": "es",
                    },
                },
                "required": ["query"],
            },
        ),
    ]


@app.call_tool()
async def call_tool(name: str, arguments: dict):
    if name == "speak":
        return await handle_speak(arguments)
    elif name == "list_languages":
        return handle_list_languages()
    elif name == "speak_and_remember":
        return await handle_speak_and_remember(arguments)
    elif name == "recall_and_speak":
        return await handle_recall_and_speak(arguments)
    return [types.TextContent(type="text", text=f"Unknown tool: {name}")]


async def handle_speak(arguments: dict) -> list:
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


def handle_list_languages() -> list:
    lang_list = "\n".join([f"  {k}: {v}" for k, v in LANGUAGES.items()])
    return [types.TextContent(type="text", text=f"Available languages:\n{lang_list}")]


async def handle_speak_and_remember(arguments: dict) -> list:
    text = arguments.get("text", "")
    lang = arguments.get("lang", "es")
    title = arguments.get("title", f"TTS: {text[:50]}...")
    importance = arguments.get("importance", 0.5)

    if not text:
        return [types.TextContent(type="text", text="Error: No text provided")]

    # Generate speech
    os.makedirs(OUTPUT_DIR, exist_ok=True)
    import hashlib
    text_hash = hashlib.md5(text.encode()).hexdigest()[:8]
    output_path = os.path.join(OUTPUT_DIR, f"speech_{text_hash}.mp3")

    try:
        tts = gTTS(text=text, lang=lang)
        tts.save(output_path)
    except Exception as e:
        return [types.TextContent(type="text", text=f"Error generating speech: {str(e)}")]

    # Store in Nexo memory
    try:
        result = subprocess.run(
            [NEXO_BIN, "store", text, "--title", title, "--importance", str(importance)],
            capture_output=True,
            text=True,
            timeout=10,
        )
        memory_status = result.stdout if result.returncode == 0 else f"Memory error: {result.stderr}"
    except Exception as e:
        memory_status = f"Memory error: {str(e)}"

    return [
        types.TextContent(
            type="text",
            text=f"✅ Audio: {output_path}\n🧠 Memory: {memory_status}",
        )
    ]


async def handle_recall_and_speak(arguments: dict) -> list:
    query = arguments.get("query", "")
    lang = arguments.get("lang", "es")

    if not query:
        return [types.TextContent(type="text", text="Error: No query provided")]

    # Recall from memory
    try:
        result = subprocess.run(
            [NEXO_BIN, "recall-brief", query],
            capture_output=True,
            text=True,
            timeout=10,
        )
        if result.returncode != 0:
            return [types.TextContent(type="text", text=f"Recall error: {result.stderr}")]
        
        # Extract key information from recall output
        recall_output = result.stdout
        # Simplify for speech
        lines = recall_output.split("\n")
        speech_text = ""
        for line in lines:
            if line.strip() and not line.startswith("╔") and not line.startswith("╚") and not line.startswith("║"):
                speech_text += line.strip() + ". "
        
        if not speech_text:
            speech_text = f"No encontré información sobre: {query}"
        
        # Truncate if too long
        if len(speech_text) > 500:
            speech_text = speech_text[:500] + "..."

    except Exception as e:
        speech_text = f"Error al recordar: {str(e)}"

    # Generate speech
    os.makedirs(OUTPUT_DIR, exist_ok=True)
    import hashlib
    text_hash = hashlib.md5(speech_text.encode()).hexdigest()[:8]
    output_path = os.path.join(OUTPUT_DIR, f"recall_{text_hash}.mp3")

    try:
        tts = gTTS(text=speech_text, lang=lang)
        tts.save(output_path)
        return [
            types.TextContent(
                type="text",
                text=f"🧠 Recall: {query}\n🔊 Audio: {output_path}\n📝 {speech_text[:200]}...",
            )
        ]
    except Exception as e:
        return [types.TextContent(type="text", text=f"Error generating speech: {str(e)}")]


async def main():
    async with stdio_server() as (read_stream, write_stream):
        await app.run(read_stream, write_stream, app.create_initialization_options())


if __name__ == "__main__":
    asyncio.run(main())
