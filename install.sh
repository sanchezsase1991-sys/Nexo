#!/bin/bash
# ═══════════════════════════════════════════════════════════════
# ⚡ Nexo — Instalación
# ═══════════════════════════════════════════════════════════════

set -euo pipefail

BOLD='\033[1m'; GREEN='\033[0;32m'; CYAN='\033[0;36m'; YELLOW='\033[1;33m'; RED='\033[0;31m'; NC='\033[0m'
OS="$(uname -s)"
ARCH="$(uname -m)"

echo -e "${BOLD}${CYAN}╔══════════════════════════════════════════╗${NC}"
echo -e "${BOLD}${CYAN}║        ⚡ NEXO INSTALL v1.0             ║${NC}"
echo -e "${BOLD}${CYAN}╚══════════════════════════════════════════╝${NC}"

# Detectar OS
case "$OS" in
    Darwin)
        CORTEX_HOME="$HOME/.memory-cortex"; NEXO_HOME="$HOME/.nexo"; BIN_DIR="/usr/local/bin"
        NEED_SUDO=false; [ ! -w "$BIN_DIR" ] && NEED_SUDO=true ;;
    Linux)
        CORTEX_HOME="/root/.memory-cortex"; NEXO_HOME="/root/.nexo"; BIN_DIR="/usr/local/bin"
        NEED_SUDO=false ;;
    *) echo -e "${RED}[✗] OS no soportado: $OS${NC}"; exit 1 ;;
esac

echo -e "\n${BOLD}Paso 1/4: Directorios${NC}"
mkdir -p "$CORTEX_HOME" "$NEXO_HOME" "$(dirname "$0")/nexo"
echo -e "  ${GREEN}✓${NC} $CORTEX_HOME\n  ${GREEN}✓${NC} $NEXO_HOME"

echo -e "\n${BOLD}Paso 2/4: Archivos de configuración${NC}"
cp "$(dirname "$0")/init.md" "$NEXO_HOME/init.md"
cp "$(dirname "$0")/cortex.sh" "$CORTEX_HOME/cortex.sh"
chmod +x "$CORTEX_HOME/cortex.sh"
echo -e "  ${GREEN}✓${NC} init.md\n  ${GREEN}✓${NC} cortex.sh"

echo -e "\n${BOLD}Paso 3/4: Compilar binario${NC}"
cd "$(dirname "$0")/nexo"
if ! command -v go &>/dev/null; then
    echo -e "  ${RED}[✗] Go no instalado. Instálalo primero:${NC}"
    echo -e "       macOS: brew install go\n       Linux: apt install golang-go"
    exit 1
fi
echo -e "  Compilando con Go..."
go build -o cortex .
if [ "$NEED_SUDO" = true ]; then
    sudo cp cortex "$BIN_DIR/cortex"
else
    cp cortex "$BIN_DIR/cortex"
fi
echo -e "  ${GREEN}✓${NC} $BIN_DIR/cortex ($(file "$BIN_DIR/cortex" | awk -F: '{print $2}'))"

echo -e "\n${BOLD}Paso 4/4: Memoria${NC}"
# Copiar graph.db si existe en el repo
if [ -f "$(dirname "$0")/graph.db" ]; then
    cp "$(dirname "$0")/graph.db" "$CORTEX_HOME/graph.db"
    echo -e "  ${GREEN}✓${NC} Memoria copiada ($(ls -lh "$CORTEX_HOME/graph.db" | awk '{print $5}'))"
else
    echo -e "  ${YELLOW}⚠  No hay graph.db en el repo. Ejecuta 'nexo init' para empezar freso.${NC}"
fi

echo -e "\n${BOLD}Shell: alias y variables${NC}"
SHELL_RC="$HOME/.zshrc"; [ "$OS" = "Linux" ] && SHELL_RC="$HOME/.bashrc" && [ -f "$HOME/.zshrc" ] && SHELL_RC="$HOME/.zshrc"
export MEMORY_CORTEX_DIR="$CORTEX_HOME"
grep -q "MEMORY_CORTEX_DIR" "$SHELL_RC" 2>/dev/null || echo "export MEMORY_CORTEX_DIR=\"\$HOME/.memory-cortex\"" >> "$SHELL_RC"
grep -q "alias nexo=" "$SHELL_RC" 2>/dev/null || echo 'alias nexo="bash $HOME/.memory-cortex/cortex.sh"' >> "$SHELL_RC"
echo -e "  ${GREEN}✓${NC} MEMORY_CORTEX_DIR + alias 'nexo' en $SHELL_RC"

echo -e "\n${BOLD}${GREEN}╔══════════════════════════════════════════╗${NC}"
echo -e "${BOLD}${GREEN}║     ✓ NEXO INSTALADO CORRECTAMENTE     ║${NC}"
echo -e "${BOLD}${GREEN}╚══════════════════════════════════════════╝${NC}"
echo -e "\n  Recarga tu shell y prueba:"
echo -e "    source $SHELL_RC"
echo -e "    nexo handoff\n"

if [ "$OS" = "Darwin" ]; then
    echo -e "${GREEN}  🍏 Nexo vive en tu Mac ahora.${NC}"
else
    echo -e "${GREEN}  🐧 Nexo instalado en Linux.${NC}"
fi
echo -e "${GREEN}  Nosotros somos un equipo.${NC}\n"
