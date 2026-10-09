#!/bin/sh
set -eu
umask 077
SOURCE=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
ROOT=/opt/etc/optiray-go
SERVICE=/opt/etc/init.d/S99optiray-go
HOOK=/opt/etc/ndm/netfilter.d/100-optiray-go-routing
LEGACY=/opt/etc/init.d/S99optiray
LEGACY_HOOK=/opt/etc/ndm/netfilter.d/100-optiray-routing
[ "$(id -u)" = 0 ] || { echo 'Run as root' >&2; exit 1; }
[ -f "$SOURCE/bin/optiray" ] || { echo 'Extract the whole Go package first' >&2; exit 1; }
case "$(uname -m)" in aarch64) ARCH=arm64;; mips*) ARCH=mipsle;; *) echo 'Unsupported router architecture' >&2; exit 1;; esac
[ "$(cat "$SOURCE/ARCH")" = "$ARCH" ] || { echo 'Wrong package architecture' >&2; exit 1; }
(cd "$SOURCE" && sha256sum -c SHA256SUMS)
chmod 755 "$SOURCE/bin/optiray" "$SOURCE/bin/optiroutd"
"$SOURCE/bin/optiray" --version
EXISTING=0
if [ -e "$ROOT" ]; then
 ROOT_PARENT=$(CDPATH= cd -P -- "$(dirname -- "$ROOT")" && pwd)
 [ "$(readlink -f "$ROOT")" = "$ROOT_PARENT/$(basename -- "$ROOT")" ] || { echo 'Refusing redirected installation directory' >&2; exit 1; }
 [ -f "$ROOT/.optiray-go-install" ] || { echo 'Target folder is not an owned Go installation' >&2; exit 1; }
 EXISTING=1
fi
if [ -e "$HOOK" ]; then grep -q 'OptiRay Go owned netfilter hook' "$HOOK" || { echo 'Foreign netfilter hook' >&2; exit 1; }; fi
if [ -e "$SERVICE" ]; then grep -q 'OptiRay Go owned service' "$SERVICE" || { echo 'Foreign service at target path' >&2; exit 1; }; fi
if [ -e "$LEGACY" ]; then grep -q 'APP=/opt/etc/optiray/app.py' "$LEGACY" || { echo 'Foreign legacy service; refusing migration' >&2; exit 1; }; fi
if [ -e "$LEGACY_HOOK" ]; then grep -q '/opt/var/run/optirout.sock' "$LEGACY_HOOK" || { echo 'Foreign legacy firewall hook' >&2; exit 1; }; fi
# Resolve dependencies before stopping anything, while the current internet works.
PACKAGES=''
for tool in iptables-save iptables-restore ip6tables-save ip6tables-restore; do
 if ! command -v "$tool" >/dev/null 2>&1; then PACKAGES="$PACKAGES iptables"; fi
done
if ! command -v ip >/dev/null 2>&1 || ! ip -V 2>/dev/null | grep -q iproute2; then PACKAGES="$PACKAGES ip-full"; fi
if [ -z "${SSL_CERT_FILE:-}" ] && [ -z "${SSL_CERT_DIR:-}" ] && [ ! -r /opt/etc/ssl/certs/ca-certificates.crt ]; then PACKAGES="$PACKAGES ca-bundle"; fi
PACKAGES=$(printf '%s\n' "$PACKAGES" | tr ' ' '\n' | sed '/^$/d' | sort -u | tr '\n' ' ')
if [ -n "$PACKAGES" ]; then
 command -v opkg >/dev/null || { echo 'Entware opkg is missing' >&2; exit 1; }
 echo "Устанавливаются зависимости Entware: $PACKAGES"
 opkg update || { echo 'Не удалось обновить списки Entware' >&2; exit 1; }
 # Package names are a fixed, deduplicated allowlist.
 opkg install $PACKAGES || { echo 'Не удалось установить зависимости; OptiRay не изменён' >&2; exit 1; }
fi
for tool in iptables-save iptables-restore ip6tables-save ip6tables-restore ip; do command -v "$tool" >/dev/null || { echo "Entware package did not provide: $tool" >&2; exit 1; }; done
if [ -z "${SSL_CERT_FILE:-}" ] && [ -z "${SSL_CERT_DIR:-}" ] && [ ! -r /opt/etc/ssl/certs/ca-certificates.crt ]; then echo 'CA bundle is still missing' >&2; exit 1; fi
LAN=$("$SOURCE/bin/optiray" --router-address)
# Keep extraction and rollback away from small Entware flash storage.
TEMP_ROOT=${OPTIRAY_INSTALL_TMPDIR:-/tmp}
case "$TEMP_ROOT" in /*) ;; *) echo 'Temporary directory must be absolute' >&2; exit 1;; esac
mkdir -p "$TEMP_ROOT"
space_kb() { df -Pk "$1" | awk 'END {print $4}'; }
size_kb() { du -sk "$1" | awk '{print $1}'; }
BACKUP_KB=0
if [ "$EXISTING" = 1 ]; then BACKUP_KB=$(size_kb "$ROOT"); fi
TEMP_FREE=$(space_kb "$TEMP_ROOT")
TEMP_NEED=$((BACKUP_KB + 8192))
[ "$TEMP_FREE" -ge "$TEMP_NEED" ] || { echo "Недостаточно места для отката в $TEMP_ROOT: нужно $TEMP_NEED КиБ, доступно $TEMP_FREE КиБ. OptiRay не остановлен." >&2; exit 1; }
NEW_KB=$(size_kb "$SOURCE")
OLD_MANAGED_KB=0
for file in optiray optiroutd; do
 if [ -f "$ROOT/bin/$file" ]; then OLD_MANAGED_KB=$((OLD_MANAGED_KB + $(size_kb "$ROOT/bin/$file"))); fi
done
TARGET_NEED=$((NEW_KB - OLD_MANAGED_KB + 4096))
[ "$TARGET_NEED" -ge 4096 ] || TARGET_NEED=4096
# A fresh legacy migration also copies private cores, geodata and settings.
if [ "$EXISTING" = 0 ] && [ -f /opt/etc/optiray/subs.json ]; then
 TARGET_NEED=$((TARGET_NEED + $(size_kb /opt/etc/optiray)))
 for file in xray hysteria geoip.dat geosite.dat; do
  if [ -f "/opt/bin/$file" ]; then TARGET_NEED=$((TARGET_NEED + $(size_kb "/opt/bin/$file"))); fi
 done
fi
TARGET_FREE=$(space_kb /opt/etc)
[ "$TARGET_FREE" -ge "$TARGET_NEED" ] || { echo "Недостаточно места в Entware: нужно $TARGET_NEED КиБ, доступно $TARGET_FREE КиБ. OptiRay не остановлен." >&2; exit 1; }
# If the temporary location is on the same filesystem, both allocations count.
TEMP_DEVICE=$(df -Pk "$TEMP_ROOT" | awk 'END {print $1}')
TARGET_DEVICE=$(df -Pk /opt/etc | awk 'END {print $1}')
if [ "$TEMP_DEVICE" = "$TARGET_DEVICE" ]; then
 TOTAL_NEED=$((TEMP_NEED + TARGET_NEED))
 [ "$TARGET_FREE" -ge "$TOTAL_NEED" ] || { echo "Недостаточно места для обновления и отката: нужно $TOTAL_NEED КиБ. OptiRay не остановлен." >&2; exit 1; }
fi
WORK=$(mktemp -d "$TEMP_ROOT/optiray-go-update.XXXXXX")
GO_WAS_RUNNING=0
LEGACY_WAS_RUNNING=0
CHANGED=0
SUCCESS=0
LEGACY_MOVED=0
LEGACY_HOOK_MOVED=0
cleanup() {
 result=$?
 trap - EXIT INT TERM HUP
 set +e
 if [ "$SUCCESS" = 0 ] && [ "$CHANGED" = 1 ]; then
  [ ! -f "$ROOT/manager.log" ] || cp "$ROOT/manager.log" "$WORK/failed-manager.log"
  [ ! -f "$ROOT/routing/engine.log" ] || cp "$ROOT/routing/engine.log" "$WORK/failed-engine.log"
  echo 'Установка не завершилась. Возвращаются прежние файлы и службы.' >&2
  if [ -f "$SERVICE" ]; then
   if ! "$SERVICE" stop; then echo "Не удалось остановить Go. Проверьте $ROOT/manager.log; сохранённые файлы: $WORK" >&2; exit 1; fi
  fi
  if [ -d "$WORK/go-before" ]; then rm -rf "$ROOT"; cp -pR "$WORK/go-before" "$ROOT"; else
   [ -f "$ROOT/.optiray-go-install" ] && rm -rf "$ROOT"
  fi
  if [ -f "$WORK/service" ]; then cp -p "$WORK/service" "$SERVICE"; else rm -f "$SERVICE"; fi
  if [ -f "$WORK/hook" ]; then cp -p "$WORK/hook" "$HOOK"; else rm -f "$HOOK"; fi
  if [ "$LEGACY_MOVED" = 1 ]; then mv "$WORK/legacy-service" "$LEGACY"; fi
  if [ "$LEGACY_HOOK_MOVED" = 1 ]; then mv "$WORK/legacy-hook" "$LEGACY_HOOK"; fi
  if [ "$GO_WAS_RUNNING" = 1 ]; then "$SERVICE" start || result=1; fi
  if [ "$LEGACY_WAS_RUNNING" = 1 ]; then "$SOURCE/bin/optiray" --data-dir "$WORK" --start-legacy || result=1; fi
 fi
 if [ "$SUCCESS" = 0 ] && [ "$CHANGED" = 0 ] && [ "$GO_WAS_RUNNING" = 1 ]; then "$SERVICE" start || result=1; fi
 if [ "$SUCCESS" = 0 ]; then
  [ ! -f "$ROOT/manager.log" ] || cp "$ROOT/manager.log" "$WORK/manager.log"
  echo "Диагностика и резервная копия установки: $WORK" >&2
 else rm -rf "$WORK"; fi
 exit "$result"
}
trap cleanup EXIT
trap 'exit 1' INT TERM HUP
if [ -f "$SERVICE" ]; then
 if "$SERVICE" status >/dev/null 2>&1; then GO_WAS_RUNNING=1; fi
 cp -p "$SERVICE" "$WORK/service"
 "$SERVICE" stop
fi
if [ -f "$HOOK" ]; then cp -p "$HOOK" "$WORK/hook"; fi
if [ "$EXISTING" = 1 ]; then cp -pR "$ROOT" "$WORK/go-before"; fi
"$SOURCE/bin/optiray" --data-dir "$WORK" --capture-legacy-auth
CHANGED=1
if [ "$("$SOURCE/bin/optiray" --legacy-status)" = running ]; then LEGACY_WAS_RUNNING=1; fi
# Disable only the verified old OptiRay boot service and its hook. Its data stays intact.
if [ -f "$LEGACY" ]; then mv "$LEGACY" "$WORK/legacy-service"; LEGACY_MOVED=1; fi
if [ -f "$LEGACY_HOOK" ]; then mv "$LEGACY_HOOK" "$WORK/legacy-hook"; LEGACY_HOOK_MOVED=1; fi
"$SOURCE/bin/optiray" --stop-legacy
mkdir -p "$ROOT/bin" "$ROOT/ui" "$ROOT/run" "$ROOT/install-state"
chmod 700 "$ROOT"
printf 'OptiRay Go\n' > "$ROOT/.optiray-go-install"
if [ "$EXISTING" = 0 ] && [ -f /opt/etc/optiray/subs.json ]; then
 if [ ! -f /opt/etc/optiray/auth.json ] && [ -f "$WORK/legacy-auth.json" ]; then cp "$WORK/legacy-auth.json" "$ROOT/auth.json"; fi
 for file in subs.json hwid.txt auth.json; do if [ -f "/opt/etc/optiray/$file" ]; then cp -p "/opt/etc/optiray/$file" "$ROOT/$file"; fi; done
 if [ -d /opt/etc/optiray/routing ]; then cp -pR /opt/etc/optiray/routing "$ROOT/"; fi
 for file in xray hysteria geoip.dat geosite.dat; do if [ -f "/opt/bin/$file" ]; then cp -p "/opt/bin/$file" "$ROOT/bin/$file"; fi; done
 echo 'Скопированы настройки, HWID и правила Python-версии.'
fi
# The service is stopped and a complete rollback snapshot exists in RAM/USB.
# Release old manager/engine blocks before copying new binaries (no .new peak).
rm -f "$ROOT/bin/optiray" "$ROOT/bin/optiroutd"
for file in optiray optiroutd; do cp "$SOURCE/bin/$file" "$ROOT/bin/$file.new"; chmod 755 "$ROOT/bin/$file.new"; mv "$ROOT/bin/$file.new" "$ROOT/bin/$file"; done
cp "$SOURCE/ui/"* "$ROOT/ui/"
cp "$SOURCE/uninstall.sh" "$ROOT/uninstall.sh"
cp "$SOURCE/"*.optirout "$ROOT/"
cp "$SOURCE/optirout-source.tar.gz" "$ROOT/"
cp "$SOURCE/S99optiray-go" "$SERVICE"
mkdir -p "$(dirname "$HOOK")"
cp "$SOURCE/100-optiray-go-routing" "$HOOK"
chmod 755 "$SERVICE" "$HOOK" "$ROOT/uninstall.sh"
"$ROOT/bin/optiray" --data-dir "$ROOT" --initialize-install
"$SERVICE" start || { echo "Запуск не удался: $ROOT/manager.log" >&2; exit 1; }
ATTEMPT=0
until "$ROOT/bin/optiray" --data-dir "$ROOT" --check-service > "$WORK/health.log" 2>&1; do
 ATTEMPT=$((ATTEMPT+1))
 if [ "$ATTEMPT" -ge 20 ]; then cat "$WORK/health.log" >&2; echo "Проверьте $ROOT/manager.log и $ROOT/routing/engine.log" >&2; exit 1; fi
 sleep 1
done
if [ "$LEGACY_MOVED" = 1 ]; then cp -p "$WORK/legacy-service" "$ROOT/install-state/python-service.disabled"; fi
if [ "$LEGACY_WAS_RUNNING" = 1 ]; then echo running > "$ROOT/install-state/python-was-running"; fi
if [ "$LEGACY_HOOK_MOVED" = 1 ]; then cp -p "$WORK/legacy-hook" "$ROOT/install-state/python-hook.disabled"; fi
if [ -f "$WORK/legacy-auth.json" ]; then cp "$WORK/legacy-auth.json" "$ROOT/install-state/legacy-auth.json"; fi
SUCCESS=1
if [ "$EXISTING" = 1 ]; then echo 'OptiRay Go обновлён и запущен.'; else echo 'OptiRay Go установлен и запущен.'; fi
echo "Интерфейс OptiRay доступен по адресу: http://$LAN:3883"
echo "Управление службой: $SERVICE start|stop|restart|status"
