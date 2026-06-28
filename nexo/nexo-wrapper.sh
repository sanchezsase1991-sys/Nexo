#!/bin/bash
# ============================================================
#  NEXO WRAPPER - Ejecutor de comandos Nexo
#  Alternativa a las herramientas MCP de OpenCode
# ============================================================

NEXO_BIN="/root/primerModelo/nexo/cortex"
DB_DIR="/root/.memory-cortex"

# Verificar que el binario existe
if [ ! -x "$NEXO_BIN" ]; then
    echo "Error: Binario nexo no encontrado en $NEXO_BIN"
    exit 1
fi

# Verificar que la base de datos existe
if [ ! -f "$DB_DIR/graph.db" ]; then
    echo "Error: Base de datos no encontrada en $DB_DIR/graph.db"
    exit 1
fi

# Ejecutar el comando
MEMORY_CORTEX_DIR="$DB_DIR" "$NEXO_BIN" "$@"
