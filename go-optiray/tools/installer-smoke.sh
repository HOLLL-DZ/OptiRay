#!/bin/sh
set -eu
TESTROOT=${1:?Provide an isolated test directory}
case "$TESTROOT" in /opt/tmp/optiray-installer-tests.*) ;; *) echo 'Unexpected test root' >&2; exit 1;; esac
BASE_PATH=$PATH
setup() {
 CASE=$TESTROOT/$1
 mkdir -p "$CASE/bin" "$CASE/source/bin" "$CASE/source/ui" "$CASE/init" "$CASE/hooks" "$CASE/python" "$CASE/tmp"
 for tool in id uname sha256sum chmod cp mv rm grep tr sed sort dirname basename readlink tar mkdir mktemp awk cat touch du; do ln -s "$(PATH=$BASE_PATH command -v "$tool")" "$CASE/bin/$tool"; done
 cat > "$CASE/bin/df" <<'DF'
#!/bin/sh
printf 'Filesystem 1024-blocks Used Available Capacity Mounted on\n'
free=${FAKE_FREE_KB:-500000}
case "$*" in *"$FAKE_CASE/tmp"*) ;; *) free=${FAKE_TARGET_FREE_KB:-500000};; esac
printf 'fake 1000000 0 %s 0%% /\n' "$free"
DF
 printf '#!/bin/sh\nexit 0\n' > "$CASE/bin/sleep"
 cat > "$CASE/bin/opkg" <<'OPKG'
#!/bin/sh
printf '%s\n' "$*" >> "$FAKE_CASE/opkg.log"
[ "${FAKE_DEP_FAIL:-0}" = 0 ] || exit 1
if [ "$1" = install ]; then
 for command in iptables-save iptables-restore ip6tables-save ip6tables-restore; do printf '#!/bin/sh\nexit 0\n' > "$FAKE_CASE/bin/$command"; chmod 755 "$FAKE_CASE/bin/$command"; done
 printf '#!/bin/sh\necho iproute2\n' > "$FAKE_CASE/bin/ip"; chmod 755 "$FAKE_CASE/bin/ip"
 echo cert > "$FAKE_CASE/cert.pem"
fi
OPKG
 cat > "$CASE/source/bin/optiray" <<'MANAGER'
#!/bin/sh
case "$*" in
 *--version*) echo 1.0.1;;
 *--router-address*) echo 192.168.10.1;;
 *--legacy-status*) if [ -f "$FAKE_CASE/legacy-running" ]; then echo running; else echo stopped; fi;;
 *--stop-legacy*) rm -f "$FAKE_CASE/legacy-running";;
 *--start-legacy*) touch "$FAKE_CASE/legacy-running";;
 *--capture-legacy-auth*) :;;
 *--initialize-install*) :;;
 *--check-service*) [ "${FAKE_HEALTH_FAIL:-0}" = 0 ];;
 *) exit 1;;
esac
MANAGER
 cat > "$CASE/source/S99optiray-go" <<'SERVICE'
#!/bin/sh
# OptiRay Go owned service
case "$1" in
 start) [ "${FAKE_START_FAIL:-0}" = 0 ] || exit 1; touch "$FAKE_CASE/go-running";;
 stop) rm -f "$FAKE_CASE/go-running";;
 status) [ -f "$FAKE_CASE/go-running" ];;
esac
SERVICE
 printf 'arm64\n' > "$CASE/source/ARCH"
 printf '#!/bin/sh\nexit 0\n' > "$CASE/source/bin/optiroutd"
 echo '# OptiRay Go owned netfilter hook' > "$CASE/source/100-optiray-go-routing"
 echo dummy > "$CASE/source/uninstall.sh"
 echo dummy > "$CASE/source/LICENSE.optirout"
 echo dummy > "$CASE/source/optirout-source.tar.gz"
 echo dummy > "$CASE/source/ui/page.html"
 # Every absolute target in the installer is redirected into this case.
 sed "s|/opt/tmp|__TEMP_PATH__|g;s|/opt/etc/optiray-go|$CASE/target|g;s|/opt/etc/init.d/S99optiray-go|$CASE/init/S99optiray-go|g;s|/opt/etc/ndm/netfilter.d/100-optiray-go-routing|$CASE/hooks/go|g;s|/opt/etc/init.d/S99optiray|$CASE/init/S99optiray|g;s|/opt/etc/ndm/netfilter.d/100-optiray-routing|$CASE/hooks/legacy|g;s|/opt/etc/optiray/|$CASE/python/|g;s|/opt/etc/optiray\([^a-z-]\)|$CASE/python\1|g;s|/opt/etc/ssl/certs/ca-certificates.crt|$CASE/cert.pem|g;s|__TEMP_PATH__|$CASE/tmp|g;s|/opt/bin/|$CASE/old-bin/|g" "$TESTROOT/install.sh" > "$CASE/source/install.sh"
 chmod 755 "$CASE/bin/"* "$CASE/source/bin/"* "$CASE/source/S99optiray-go"
 (cd "$CASE/source"; sha256sum ARCH install.sh bin/optiray bin/optiroutd > SHA256SUMS)
}
run_case() { OPTIRAY_INSTALL_TMPDIR=$CASE/tmp FAKE_TARGET_FREE_KB=${FAKE_TARGET_FREE_KB:-500000} FAKE_FREE_KB=${FAKE_FREE_KB:-500000} FAKE_CASE=$CASE FAKE_DEP_FAIL=${FAKE_DEP_FAIL:-0} FAKE_START_FAIL=${FAKE_START_FAIL:-0} FAKE_HEALTH_FAIL=${FAKE_HEALTH_FAIL:-0} PATH=$CASE/bin /bin/sh "$CASE/source/install.sh" > "$CASE/result.log" 2>&1; }
existing() {
 mkdir -p "$CASE/target/bin"
 echo marker > "$CASE/target/.optiray-go-install"
 echo preserved > "$CASE/target/subs.json"
 echo previous > "$CASE/target/bin/optiray"
 # Old service must start normally even when the new one is forced to fail.
 sed '/FAKE_START_FAIL/s/\[.*exit 1; //' "$CASE/source/S99optiray-go" > "$CASE/init/S99optiray-go"
 chmod 755 "$CASE/init/S99optiray-go"
 touch "$CASE/go-running"
}
legacy() {
 printf '#!/bin/sh\nAPP=%s/python/app.py\n' "$CASE" > "$CASE/init/S99optiray"
 echo '/opt/var/run/optirout.sock' > "$CASE/hooks/legacy"
 echo old-settings > "$CASE/python/subs.json"
 touch "$CASE/legacy-running"
}
setup fresh
run_case
[ -f "$CASE/go-running" ]
grep -q 'iptables' "$CASE/opkg.log"
grep -q 'ip-full' "$CASE/opkg.log"
grep -q 'ca-bundle' "$CASE/opkg.log"
grep -q 'http://192.168.10.1:3883' "$CASE/result.log"
echo FRESH_DEPENDENCIES_ADDRESS_AUTOSTART_PASS
setup update
existing
run_case
[ "$(cat "$CASE/target/subs.json")" = preserved ]
echo UPDATE_SETTINGS_PASS
setup deps-fail
existing
FAKE_DEP_FAIL=1
if run_case; then exit 1; fi
FAKE_DEP_FAIL=0
[ -f "$CASE/go-running" ]
[ "$(cat "$CASE/target/bin/optiray")" = previous ]
echo DEPENDENCY_FAILURE_NO_SERVICE_CHANGE_PASS
setup start-fail
existing
FAKE_START_FAIL=1
if run_case; then exit 1; fi
FAKE_START_FAIL=0
[ -f "$CASE/go-running" ]
[ "$(cat "$CASE/target/bin/optiray")" = previous ]
echo UPDATE_START_FAILURE_ROLLBACK_PASS
setup migrate
legacy
run_case
[ ! -e "$CASE/init/S99optiray" ]
[ -f "$CASE/target/install-state/python-service.disabled" ]
[ "$(cat "$CASE/target/subs.json")" = old-settings ]
[ -f "$CASE/go-running" ]
echo LEGACY_MIGRATION_PASS
setup migrate-fail
legacy
FAKE_HEALTH_FAIL=1
if run_case; then exit 1; fi
FAKE_HEALTH_FAIL=0
[ -f "$CASE/init/S99optiray" ]
[ -f "$CASE/hooks/legacy" ]
[ -f "$CASE/legacy-running" ]
[ ! -d "$CASE/target" ]
! grep -q 'Интерфейс OptiRay доступен' "$CASE/result.log"
echo HEALTH_FAILURE_LEGACY_ROLLBACK_PASS

setup space-fail
existing
FAKE_FREE_KB=1
if run_case; then exit 1; fi
FAKE_FREE_KB=500000
[ -f "$CASE/go-running" ]
[ "$(cat "$CASE/target/bin/optiray")" = previous ]
echo SPACE_FAILURE_NO_SERVICE_CHANGE_PASS

setup target-space-fail
existing
FAKE_TARGET_FREE_KB=1
if run_case; then exit 1; fi
FAKE_TARGET_FREE_KB=500000
[ -f "$CASE/go-running" ]
[ "$(cat "$CASE/target/bin/optiray")" = previous ]
echo TARGET_SPACE_FAILURE_NO_SERVICE_CHANGE_PASS
