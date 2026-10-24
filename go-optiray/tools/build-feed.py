"""Build Entware IPKs and feed indexes from existing release tarballs."""
import argparse
import gzip
import hashlib
import io
import posixpath
from pathlib import Path
import tarfile

PREINST = r'''#!/bin/sh
set -eu
case "${IPKG_INSTROOT:-}" in ''|/) ;; *) exit 0;; esac
ROOT=/opt/etc/optiray-go
SERVICE=/opt/etc/init.d/S99optiray-go
STATE=${OPTIRAY_BACKUP_DIR:-/opt/tmp/optiray-opkg-state}
POINTER=/opt/var/lib/optiray/opkg-backup-path
case "$STATE" in /*) ;; *) echo 'Backup path must be absolute' >&2; exit 1;; esac
PARENT=$(dirname "$STATE")
mkdir -p "$PARENT"
PARENT=$(cd "$PARENT" && pwd -P)
STATE="$PARENT/$(basename "$STATE")"
case "$STATE" in /|/opt|/opt/etc|/opt/etc/optiray-go|/opt/etc/optiray-go/*|*/..|*/.) echo 'Unsafe backup path' >&2; exit 1;; esac
case "$ROOT/" in "$STATE/"*) echo 'Backup cannot contain the application directory' >&2; exit 1;; esac
[ ! -L "$STATE" ] || exit 1
[ ! -e "$POINTER" ] || { echo "Previous opkg operation needs recovery; see $POINTER" >&2; exit 1; }
[ ! -e /tmp/optiray-opkg-state ] || { echo 'Previous recovery backup exists in /tmp/optiray-opkg-state' >&2; exit 1; }
[ ! -e "$STATE" ] || { echo "Previous opkg operation needs recovery: $STATE" >&2; exit 1; }
if [ -e /opt/etc/init.d/S99optiray ]; then
 echo 'Migrate the Python installation using the Go installer before installing this package.' >&2
 exit 1
fi
if [ -e "$ROOT" ]; then
 [ -f "$ROOT/.optiray-go-install" ] || { echo 'Unowned installation directory' >&2; exit 1; }
 [ ! -L "$ROOT" ] || exit 1
fi
if [ -e "$SERVICE" ]; then grep -q 'OptiRay Go owned service' "$SERVICE" || exit 1; fi
HOOK=/opt/etc/ndm/netfilter.d/100-optiray-go-routing
if [ -e "$HOOK" ]; then grep -q 'OptiRay Go owned netfilter hook' "$HOOK" || exit 1; fi
LIST=$(mktemp "$PARENT/optiray-backup-list.XXXXXX")
trap 'rm -f "$LIST"' EXIT
if [ -d "$ROOT" ]; then
 (cd "$ROOT" && find . \( -name log -o -name logs -o -name cache -o -name tmp -o -name run -o -name '*.log' -o -name '*.log.*' -o -name '*_cache.json' -o -name engine.pid -o -name engine.sock \) -prune -o \( -type f -o -type l \) -print) > "$LIST"
fi
NEED=8192
while IFS= read -r entry; do
 SIZE=$(du -k "$ROOT/$entry" | awk 'END {print $1}')
 NEED=$((NEED + SIZE))
done < "$LIST"
FREE=$(df -Pk "$PARENT" | awk 'END {print $4}')
[ "$FREE" -ge "$NEED" ] || { echo "Not enough space for update backup on $PARENT (need $NEED KiB, free $FREE KiB)" >&2; exit 1; }
umask 077
mkdir "$STATE"
touch "$STATE/.optiray-opkg-backup"
mkdir -p "$(dirname "$POINTER")"
printf '%s\n' "$STATE" > "$POINTER"
if [ -f "$SERVICE" ]; then
 cp -p "$SERVICE" "$STATE/service"
 if "$SERVICE" status >/dev/null 2>&1; then touch "$STATE/running"; fi
 "$SERVICE" stop
fi
if [ -d "$ROOT" ]; then
 mkdir "$STATE/root"
 while IFS= read -r entry; do
  mkdir -p "$STATE/root/$(dirname "$entry")"
  cp -pR "$ROOT/$entry" "$STATE/root/$entry"
 done < "$LIST"
 touch "$STATE/complete"
fi
if [ -e "$HOOK" ]; then
 cp -p "$HOOK" "$STATE/hook"
fi
rm -f "$LIST"
trap - EXIT
'''

POSTINST = r'''#!/bin/sh
set -eu
case "${IPKG_INSTROOT:-}" in ''|/) ;; *) exit 0;; esac
ROOT=/opt/etc/optiray-go
SERVICE=/opt/etc/init.d/S99optiray-go
STATE=${OPTIRAY_BACKUP_DIR:-/opt/tmp/optiray-opkg-state}
POINTER=/opt/var/lib/optiray/opkg-backup-path
if [ -f "$POINTER" ]; then STATE=$(cat "$POINTER"); fi
case "$STATE" in /*) ;; *) echo 'Invalid recovery path' >&2; exit 1;; esac
case "$STATE" in /|/opt|/opt/etc|/opt/etc/optiray-go|/opt/etc/optiray-go/*|*/..|*/.) echo 'Unsafe recovery path' >&2; exit 1;; esac
[ ! -L "$STATE" ] || exit 1
EXPECTED='@VERSION@'
# Validate the delivered files before arming rollback. A repeated configure after
# rollback must fail without deleting the recovery copy or accepting old code.
DISK_VERSION=$(cat "$ROOT/VERSION" 2>/dev/null || true)
BINARY_VERSION=$("$ROOT/bin/optiray" --version 2>/dev/null || true)
if [ "$DISK_VERSION" != "$EXPECTED" ] || [ "$BINARY_VERSION" != "$EXPECTED" ]; then
 echo "Package expects OptiRay $EXPECTED but files/binary are $DISK_VERSION/$BINARY_VERSION. Reinstall the corrected package; preserve $STATE." >&2
 exit 1
fi
recover() {
 if ! "$SERVICE" stop >/dev/null 2>&1; then
  echo "Service could not stop; backup preserved at $STATE" >&2
  return
 fi
 if [ -f "$STATE/complete" ] && [ -d "$STATE/root" ]; then
  cp -pR "$STATE/root/." "$ROOT/"
  [ ! -f "$STATE/service" ] || cp -p "$STATE/service" "$SERVICE"
  [ ! -f "$STATE/hook" ] || cp -p "$STATE/hook" /opt/etc/ndm/netfilter.d/100-optiray-go-routing
  if [ -f "$STATE/running" ]; then "$SERVICE" start || true; fi
 fi
 echo "Package configuration failed. Recovery backup: $STATE. Keep this backup and reinstall the package after fixing the cause." >&2
}
trap recover EXIT
printf '%s\n' 'OptiRay Go opkg installation' > "$ROOT/.optiray-go-install"
LAN=$("$ROOT/bin/optiray" --router-address)
"$ROOT/bin/optiray" --data-dir "$ROOT" --initialize-install
"$SERVICE" start
"$ROOT/bin/optiray" --data-dir "$ROOT" --check-service
trap - EXIT
if [ -f "$STATE/.optiray-opkg-backup" ]; then
 rm -rf "$STATE"
 rm -f "$POINTER"
fi
echo "Интерфейс OptiRay доступен по адресу: http://$LAN:3883"
'''

PRERM = r'''#!/bin/sh
set -eu
case "${IPKG_INSTROOT:-}" in ''|/) ;; *) exit 0;; esac
[ "${1:-}" = remove ] || exit 0
if [ -f /opt/etc/optiray-go/.optiray-go-install ]; then
 SERVICE=/opt/etc/init.d/S99optiray-go
 if [ -f "$SERVICE" ]; then
  grep -q 'OptiRay Go owned service' "$SERVICE" || exit 1
  "$SERVICE" stop
 fi
 if [ -x /opt/etc/optiray-go/bin/optiray ]; then
  /opt/etc/optiray-go/bin/optiray --data-dir /opt/etc/optiray-go --remove-managed-proxy
 fi
fi
'''

def archive(entries):
    out = io.BytesIO()
    with tarfile.open(fileobj=out, mode='w:gz', format=tarfile.USTAR_FORMAT) as tar:
        directories = set()
        for name, _, _ in entries:
            parent = posixpath.dirname(name)
            while parent not in ('', '.'):
                directories.add(parent)
                parent = posixpath.dirname(parent)
        for name in sorted(directories, key=lambda p: (p.count('/'), p)):
            directory = tarfile.TarInfo(name + '/')
            directory.type, directory.mode, directory.mtime = tarfile.DIRTYPE, 0o755, 0
            tar.addfile(directory)
        for name, data, mode in entries:
            member = tarfile.TarInfo(name)
            member.size, member.mode, member.mtime = len(data), mode, 0
            tar.addfile(member, io.BytesIO(data))
    return out.getvalue()

def build(source, output, version, revision):
    package_version = f"{version}-{revision}"
    output.mkdir(parents=True, exist_ok=True)
    records = []
    for target, abi in [('arm64', 'aarch64-3.10'), ('mipsle', 'mipsel-3.4')]:
        entries = []
        with tarfile.open(source / f'OptiRay-Go-{version}-{target}.tar.gz') as tar:
            for member in tar.getmembers():
                name = member.name.removeprefix('./')
                if not member.isfile() or name in ('install.sh', 'SHA256SUMS', 'VALIDATION.md'):
                    continue
                if name == 'S99optiray-go':
                    path = 'opt/etc/init.d/' + name
                elif name == '100-optiray-go-routing':
                    path = 'opt/etc/ndm/netfilter.d/' + name
                else:
                    path = 'opt/etc/optiray-go/' + name
                mode = 0o755 if name.startswith('bin/') or name in ('S99optiray-go', '100-optiray-go-routing', 'uninstall.sh') else 0o644
                entries.append(('./' + path, tar.extractfile(member).read(), mode))
        control = (f'Package: optiray\nVersion: {package_version}\nArchitecture: {abi}\n'
                   'Maintainer: HOLLL-DZ\nSection: net\nPriority: optional\n'
                   'Depends: curl, ca-bundle, wget-ssl, iptables, ip-full\n'
                   'License: GPL-3.0\n'
                   'Homepage: https://github.com/HOLLL-DZ/OptiRay\n'
                   f'Installed-Size: {sum(len(d) for _, d, _ in entries)}\n'
                   'Description: Go manager for Xray, Hysteria and OptiRout on Keenetic\n')
        control_data = archive([('./control', control.encode(), 0o644),
                                ('./preinst', PREINST.encode(), 0o755),
                                ('./postinst', POSTINST.replace('@VERSION@', version).encode(), 0o755),
                                ('./prerm', PRERM.encode(), 0o755)])
        ipk = archive([('debian-binary', b'2.0\n', 0o644),
                       ('control.tar.gz', control_data, 0o644),
                       ('data.tar.gz', archive(entries), 0o644)])
        filename = f'optiray_{package_version}_{abi}.ipk'
        (output / filename).write_bytes(ipk)
        records.append(control + f'Filename: {filename}\nSize: {len(ipk)}\n'
                       f'MD5Sum: {hashlib.md5(ipk).hexdigest()}\n'
                       f'SHA256sum: {hashlib.sha256(ipk).hexdigest()}\n\n')
    index = ''.join(records).encode()
    (output / 'Packages').write_bytes(index)
    (output / 'Packages.gz').write_bytes(gzip.compress(index, mtime=0))

if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('--source', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    parser.add_argument('--version', required=True)
    parser.add_argument('--revision', type=int, default=1)
    args = parser.parse_args()
    build(args.source, args.output, args.version, args.revision)
