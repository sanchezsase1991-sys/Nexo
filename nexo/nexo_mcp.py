#!/usr/bin/env python3
"""
Nexo Memory MCP Server
Provides memory recall, store, and handoff functions via MCP
"""

import asyncio
import os
import subprocess
import json
from pathlib import Path

from mcp import types
from mcp.server import Server
from mcp.server.stdio import stdio_server


app = Server("nexo-memory")

# Nexo binary path
NEXO_BIN = os.environ.get("NEXO_BIN", "/usr/local/bin/cortex")


def run_nexo(cmd, args=None):
    """Run a nexo command and return output"""
    full_cmd = [NEXO_BIN, cmd]
    if args:
        full_cmd.extend(args)
    
    try:
        result = subprocess.run(
            full_cmd,
            capture_output=True,
            text=True,
            timeout=30,
        )
        return result.stdout, result.stderr, result.returncode
    except subprocess.TimeoutExpired:
        return "", "Command timed out", 1
    except Exception as e:
        return "", str(e), 1


@app.list_tools()
async def list_tools():
    return [
        types.Tool(
            name="handoff",
            description="Wake up Nexo and load full context (identity, memory, preferences)",
            inputSchema={
                "type": "object",
                "properties": {},
            },
        ),
        types.Tool(
            name="recall",
            description="Recall associative memory context for a query",
            inputSchema={
                "type": "object",
                "properties": {
                    "query": {
                        "type": "string",
                        "description": "Query to recall from memory",
                    },
                    "brief": {
                        "type": "boolean",
                        "description": "Return brief format (default: false)",
                        "default": False,
                    },
                },
                "required": ["query"],
            },
        ),
        types.Tool(
            name="store",
            description="Store text in memory as an episode",
            inputSchema={
                "type": "object",
                "properties": {
                    "text": {
                        "type": "string",
                        "description": "Text to store in memory",
                    },
                    "title": {
                        "type": "string",
                        "description": "Title for the episode",
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
            name="reflect",
            description="Create a reflection linked to current context",
            inputSchema={
                "type": "object",
                "properties": {
                    "text": {
                        "type": "string",
                        "description": "Reflection text/insight",
                    },
                },
                "required": ["text"],
            },
        ),
        types.Tool(
            name="journal",
            description="Save session journal and consolidate PWS preferences",
            inputSchema={
                "type": "object",
                "properties": {},
            },
        ),
        types.Tool(
            name="prefs",
            description="Show user preferences (PWS)",
            inputSchema={
                "type": "object",
                "properties": {},
            },
        ),
        types.Tool(
            name="stats",
            description="Show memory graph statistics",
            inputSchema={
                "type": "object",
                "properties": {},
            },
        ),
    ]


@app.call_tool()
async def call_tool(name: str, arguments: dict):
    if name == "handoff":
        stdout, stderr, code = run_nexo("handoff")
        if code == 0:
            return [types.TextContent(type="text", text=stdout)]
        return [types.TextContent(type="text", text=f"Error: {stderr}")]
    
    elif name == "recall":
        query = arguments.get("query", "")
        brief = arguments.get("brief", False)
        cmd = "recall-brief" if brief else "recall"
        stdout, stderr, code = run_nexo(cmd, [query])
        if code == 0:
            return [types.TextContent(type="text", text=stdout)]
        return [types.TextContent(type="text", text=f"Error: {stderr}")]
    
    elif name == "store":
        text = arguments.get("text", "")
        title = arguments.get("title", "")
        importance = arguments.get("importance", 0.5)
        
        args = [text]
        if title:
            args.extend(["--title", title])
        args.extend(["--importance", str(importance)])
        
        stdout, stderr, code = run_nexo("store", args)
        if code == 0:
            return [types.TextContent(type="text", text=stdout)]
        return [types.TextContent(type="text", text=f"Error: {stderr}")]
    
    elif name == "reflect":
        text = arguments.get("text", "")
        stdout, stderr, code = run_nexo("reflect", [text])
        if code == 0:
            return [types.TextContent(type="text", text=stdout)]
        return [types.TextContent(type="text", text=f"Error: {stderr}")]
    
    elif name == "journal":
        stdout, stderr, code = run_nexo("journal")
        if code == 0:
            return [types.TextContent(type="text", text=stdout)]
        return [types.TextContent(type="text", text=f"Error: {stderr}")]
    
    elif name == "prefs":
        stdout, stderr, code = run_nexo("prefs")
        if code == 0:
            return [types.TextContent(type="text", text=stdout)]
        return [types.TextContent(type="text", text=f"Error: {stderr}")]
    
    elif name == "stats":
        stdout, stderr, code = run_nexo("stats")
        if code == 0:
            return [types.TextContent(type="text", text=stdout)]
        return [types.TextContent(type="text", text=f"Error: {stderr}")]
    
    return [types.TextContent(type="text", text=f"Unknown tool: {name}")]


async def main():
    async with stdio_server() as (read_stream, write_stream):
        await app.run(read_stream, write_stream, app.create_initialization_options())


if __name__ == "__main__":
    asyncio.run(main())
