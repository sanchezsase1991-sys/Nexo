#!/bin/bash
# ============================================================
#  MEMORY CORTEX v3 — Wrapper Go
#  Motor de memoria asociativa en Go (binario: nexo)
#  Compatible hacia atrás con cortex.sh v2
# ============================================================
#  Este script es un wrapper ligero que delega toda la
#  funcionalidad al binario compilado `nexo`.
#  Original en: /root/.memory-cortex/nexo/
# ============================================================

NEXO_BIN="/usr/local/bin/cortex"
HANDOFF_MARKER="/tmp/.nexo-handoff-done"

# Si el binario no existe, caer al shell original
if [ ! -x "$NEXO_BIN" ]; then
    exec bash "$(dirname "$0")/cortex.sh.bak" "$@"
fi

# Mapeo de comandos compatibles
case "${1:-}" in
    propagate)
        shift
        exec "$NEXO_BIN" propagate "$@"
        ;;
    entities)
        shift
        exec "$NEXO_BIN" entities "$@"
        ;;
    migrate)
        echo "⚠️  migrate: usa 'nexo init' para BD nueva, o conserva la existente"
        echo "   La BD existente (graph.db) es 100% compatible con nexo"
        exit 0
        ;;
    recall-brief)
        # 🌅 PROTOCOLO DE DESPERTAR AUTOMÁTICO
        # En cada llamada a recall-brief, se incluye el estado consolidado
        # Esto permite que Nexo "despierte sabiendo" sin iteraciones extra
        "$NEXO_BIN" handoff 2>/dev/null
        shift
        exec "$NEXO_BIN" recall-brief "$@"
        ;;
    *)
        # Todos los demás comandos pasan directo
        exec "$NEXO_BIN" "$@"
        ;;
esac
