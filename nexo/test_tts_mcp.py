#!/usr/bin/env python3
"""
Nexo TTS MCP Test Script
Demonstrates the TTS MCP server capabilities
"""

import asyncio
from mcp.client.stdio import stdio_client, StdioServerParameters
from mcp import ClientSession


async def test_tts_mcp():
    server_params = StdioServerParameters(
        command="python3",
        args=["/root/primerModelo/nexo/tts_server.py"],
    )
    
    print("🔌 Connecting to Nexo TTS MCP Server...")
    
    async with stdio_client(server_params) as (read, write):
        async with ClientSession(read, write) as session:
            # Initialize
            await session.initialize()
            print("✅ Connected!\n")
            
            # List tools
            tools = await session.list_tools()
            print("🛠️  Available tools:")
            for tool in tools.tools:
                print(f"   • {tool.name}: {tool.description}")
            print()
            
            # Test 1: Basic speak
            print("📢 Test 1: Basic TTS...")
            result = await session.call_tool("speak", {
                "text": "Hola S, soy Nexo. El sistema de voz MCP está funcionando correctamente.",
                "lang": "es"
            })
            for content in result.content:
                print(f"   {content.text}")
            print()
            
            # Test 2: List languages
            print("🌍 Test 2: Available languages...")
            result = await session.call_tool("list_languages", {})
            for content in result.content:
                print(f"   {content.text}")
            print()
            
            # Test 3: Speak and remember
            print("🧠 Test 3: TTS + Memory...")
            result = await session.call_tool("speak_and_remember", {
                "text": "Este es un mensaje de prueba del sistema TTS integrado con la memoria de Nexo.",
                "lang": "es",
                "title": "Prueba TTS MCP",
                "importance": 0.7
            })
            for content in result.content:
                print(f"   {content.text}")
            print()
            
            # Test 4: English TTS
            print("🇬🇧 Test 4: English TTS...")
            result = await session.call_tool("speak", {
                "text": "Hello S, this is Nexo. The MCP voice system is working perfectly.",
                "lang": "en"
            })
            for content in result.content:
                print(f"   {content.text}")
            print()
            
            print("✅ All tests completed!")


if __name__ == "__main__":
    asyncio.run(test_tts_mcp())
