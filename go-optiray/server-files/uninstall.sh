#!/bin/sh
set -eu
ROOT=/opt/etc/optiray-go
SERVICE=/opt/etc/init.d/S99optiray-go
HOOK=/opt/etc/ndm/netfilter.d/100-optiray-go-routing
[ "$(id -u)" = 0 ] || { echo 'Run as root' >&2; exit 1; }
[ -f "$ROOT/.optiray-go-install" ] || { echo 'Go installation marker is missing' >&2; exit 1; }
ROOT_PARENT=$(CDPATH= cd -P -- "$(dirname -- "$ROOT")" && pwd)
[ "$(readlink -f "$ROOT")" = "$ROOT_PARENT/$(basename -- "$ROOT")" ] || { echo 'Refusing a redirected installation directory' >&2; exit 1; }
if [ -e "$HOOK" ]; then grep -q 'OptiRay Go owned netfilter hook' "$HOOK" || { echo 'Foreign netfilter hook' >&2; exit 1; }; fi
if [ -f "$SERVICE" ]; then
    grep -q 'OptiRay Go owned service' "$SERVICE" || { echo 'Foreign service; refusing deletion' >&2; exit 1; }
    "$SERVICE" stop
fi
[ ! -f "$ROOT/shutdown-error" ] || { echo 'Routing cleanup error; inspect shutdown-error before removal' >&2; exit 1; }
[ ! -e "$ROOT/routing/engine.pid" ] || { echo 'Routing PID remains; inspect routing before removal' >&2; exit 1; }
"$ROOT/bin/optiray" --data-dir "$ROOT" --remove-managed-proxy
# Restore saved legacy startup files only if no other service now occupies the path.
LEGACY=/opt/etc/init.d/S99optiray
LEGACY_HOOK=/opt/etc/ndm/netfilter.d/100-optiray-routing
if [ -f "$ROOT/install-state/python-service.disabled" ]; then
 [ ! -e "$LEGACY" ] || { echo 'Legacy service path is occupied; refusing overwrite' >&2; exit 1; }
 grep -q 'APP=/opt/etc/optiray/app.py' "$ROOT/install-state/python-service.disabled" || exit 1
fi
if [ -f "$ROOT/install-state/python-hook.disabled" ]; then
 [ ! -e "$LEGACY_HOOK" ] || { echo 'Legacy hook path is occupied; refusing overwrite' >&2; exit 1; }
fi
if [ -f "$ROOT/install-state/python-service.disabled" ]; then cp -p "$ROOT/install-state/python-service.disabled" "$LEGACY"; fi
if [ -f "$ROOT/install-state/python-hook.disabled" ]; then cp -p "$ROOT/install-state/python-hook.disabled" "$LEGACY_HOOK"; fi
if [ -f "$ROOT/install-state/python-was-running" ]; then "$ROOT/bin/optiray" --data-dir "$ROOT/install-state" --start-legacy; fi
rm -f "$SERVICE" "$HOOK"
rm -rf "$ROOT"
echo 'OptiRay Go removed. No Python packages were installed or removed.'
