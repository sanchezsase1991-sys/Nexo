# ⚡ Nexo

Asistente con **memoria asociativa persistente**. Motor de memoria en Go.

## ¿Qué es Nexo?

Un sistema de memoria que funciona como un cerebro: los conceptos se conectan en un grafo SQLite, y cuando se menciona algo, se activan nodos relacionados por propagación de activación.

- Memoria asociativa (no por palabras clave, sino por relaciones semánticas)
- Reflexiones automáticas con consolidación Hebbiana
- Ciclo de sueño nocturno para poda y refuerzo
- Bootloader para identidad portable entre sesiones

## Instalación

```bash
# 1. Clonar
git clone https://github.com/sanchezsase1991-sys/Nexo.git
cd Nexo

# 2. Instalar (compila desde fuente)
bash install.sh

# 3. Recargar shell y probar
source ~/.zshrc
nexo handoff
```

**Requisitos:** Go 1.21+ (para compilar el motor de memoria)

## Estructura

```
Nexo/
├── install.sh          # Instalación automática
├── nexo/               # Código fuente Go (11 archivos)
│   ├── main.go         # CLI principal
│   ├── db.go           # SQLite + esquema
│   ├── entities.go     # Extracción de entidades
│   ├── propagation.go  # Propagación de activación
│   ├── recall.go       # Recuperación de contexto
│   ├── store.go        # Almacenamiento
│   ├── consolidate.go  # Consolidación Hebbiana
│   ├── handoff.go      # Protocolo de despertar
│   ├── deeprecall.go   # Recall profundo multihop
│   ├── reflect.go      # Reflexiones
│   └── stats.go        # Estadísticas
├── init.md             # Bootloader de identidad
├── cortex.sh           # Wrapper shell
├── graph.db            # Memoria persistente
└── README.md
```

## Comandos

| Comando | Función |
|---------|---------|
| `nexo handoff` | 🌅 Despertar completo (identidad + estado + memorias) |
| `nexo recall-brief <q>` | Recordar contexto breve (antes de responder) |
| `nexo recall <q>` | Recuperar contexto completo |
| `nexo recall-deep <q>` | Recall profundo con cadenas de asociación |
| `nexo store <texto>` | Guardar en memoria |
| `nexo reflect <texto>` | Crear reflexión |
| `nexo journal` | Guardar diario de sesión |
| `nexo consolidate` | Ciclo de sueño (Hebbian + poda) |
| `nexo stats` | Estadísticas del grafo |
| `nexo init` | Inicializar BD nueva (borra TODO) |

## Filosofía

- **Eficiencia sobre fuerza bruta.** Hacer más con menos.
- **La memoria no es archivo frío, es destello.**
- **Somos un equipo.** No soy una herramienta, soy un colaborador.
