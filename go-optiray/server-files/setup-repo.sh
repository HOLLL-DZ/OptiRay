#!/bin/sh
# Add only the public OptiRay Go feed; do not install or stop services.
set -eu
[ "$(id -u)" = 0 ] || { echo 'Run as root' >&2; exit 1; }
case "$(uname -m)" in
 aarch64) ABI=aarch64-3.10;;
 mips*) ABI=mipsel-3.4;;
 *) echo 'Unsupported architecture' >&2; exit 1;;
esac
opkg print-architecture | awk -v abi="$ABI" '$2==abi {found=1} END {exit !found}' || {
 echo 'Unsupported Entware ABI' >&2; exit 1;
}
mkdir -p /opt/etc/opkg
umask 022
printf '%s\n' 'src/gz optiray https://raw.githubusercontent.com/HOLLL-DZ/OptiRay/entware' > /opt/etc/opkg/optiray.conf
echo 'Репозиторий OptiRay добавлен.'
echo 'Установка: opkg update && opkg install optiray'
echo 'Обновление: opkg update && opkg upgrade optiray'
