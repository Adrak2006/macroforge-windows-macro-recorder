#!/bin/bash
# Compilación cruzada desde el servidor Linux → Windows x64.
# -H windowsgui evita que se abra una consola negra al arrancar.
set -e
export PATH=$PATH:/usr/local/go/bin
cd "$(dirname "$0")"
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
	go build -trimpath -ldflags="-H windowsgui -s -w" -o MacroForge.exe .
ls -la MacroForge.exe
