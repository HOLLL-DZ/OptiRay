#!/bin/sh
# Prepare HTTPS downloads and register the public OptiRay feed.
set -eu
export PATH="/opt/bin:/opt/sbin:$PATH"
[ "$(id -u)" = 0 ] || { echo 'Run as root' >&2; exit 1; }
# Limit network waits without depending on a timeout package on clean Entware.
BOOTSTRAP_TMP=$(mktemp -d /tmp/optiray-repo.XXXXXX)
trap 'rm -rf "$BOOTSTRAP_TMP"' EXIT
trap 'exit 1' INT TERM HUP
cat > "$BOOTSTRAP_TMP/wget" <<'WGET'
#!/bin/sh
exec /opt/bin/wget -T 30 -t 2 "$@"
WGET
chmod 700 "$BOOTSTRAP_TMP/wget"
export PATH="$BOOTSTRAP_TMP:/opt/bin:/opt/sbin:$PATH"
run_opkg() {
 if opkg "$@"; then return 0; else
  code=$?
  echo "opkg $* failed (exit $code). Check the last repository URL; no further installation will run." >&2
  return "$code"
 fi
}
case "$(uname -m)" in
 aarch64) ABI=aarch64-3.10;;
 mips*) ABI=mipsel-3.4;;
 *) echo 'Unsupported architecture' >&2; exit 1;;
esac
opkg print-architecture | awk -v abi="$ABI" '$2==abi {found=1} END {exit !found}' || { echo 'Unsupported Entware ABI' >&2; exit 1; }
command -v curl >/dev/null || { echo 'Install curl first. If HTTP hangs, check Entware HTTPS connectivity before retrying.' >&2; exit 1; }
# Verify the selected Entware wget supports HTTPS before changing repositories.
if ! /opt/bin/wget --version 2>/dev/null | grep -q '+https'; then
 run_opkg update
 run_opkg install wget-ssl ca-bundle
fi
for config in /opt/etc/opkg.conf /opt/etc/opkg/*.conf; do
 [ -f "$config" ] || continue
 if grep -q 'http://bin.entware.net/' "$config"; then
  echo "Переключение репозиториев Entware на HTTPS; исходный файл будет сохранён: $config"
  for address in $(awk '$1 ~ /^src/ {print $3}' "$config" | grep '^http://bin.entware.net/'); do
   secure=$(printf '%s' "$address" | sed 's|^http:|https:|')
   curl -fLsS --connect-timeout 5 --max-time 15 "$secure/Packages.gz" -o /dev/null || { echo "Cannot reach $secure; configuration unchanged." >&2; exit 1; }
   /opt/bin/wget -q -T 30 -t 2 -O /dev/null "$secure/Packages.gz" || { echo "HTTPS wget failed: $secure" >&2; exit 1; }
  done
  backup="$config.before-optiray-https-$(date +%Y%m%d-%H%M%S)"
  cp -p "$config" "$backup"
  sed 's|http://bin.entware.net/|https://bin.entware.net/|g' "$config" > "$config.optiray-new"
  cat "$config.optiray-new" > "$config"
  rm -f "$config.optiray-new"
  echo "HTTPS enabled: $config (backup: $backup)"
 fi
done
run_opkg update
run_opkg install curl ca-bundle wget-ssl
REPO=https://raw.githubusercontent.com/HOLLL-DZ/OptiRay/entware
curl -fLsS --connect-timeout 5 --max-time 20 "$REPO/Packages.gz" -o /dev/null || { echo "GitHub repository unavailable: $REPO" >&2; exit 1; }
mkdir -p /opt/etc/opkg
umask 022
printf '%s\n' "src/gz optiray $REPO" > /opt/etc/opkg/optiray.conf
echo 'Репозиторий OptiRay добавлен.'
echo 'Установка: opkg update && opkg install optiray'
echo 'Обновление: opkg update && opkg upgrade optiray'