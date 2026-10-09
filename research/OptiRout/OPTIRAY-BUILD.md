OptiRout, based on upstream MagiTrickle tag 0.8.2.
GPLv3; original authors in CONTRIBUTORS.md, license in LICENSE.
Modified JSON storage, naming, atomic writes, and Unix-only API.
Build with Go 1.23.12, CGO_ENABLED=0, GOOS=linux, GOARCH=arm64 (aarch64-3.10_kn).
For mipsel-3.4_kn use GOARCH=mipsle and GOMIPS=softfloat:
  go build -mod=vendor -tags entware -trimpath -ldflags "-X optirout/constant.Version=0.8.2-optirout -w -s" -o ../../optiroutd ./cmd/optiroutd
Host tests before setting cross-compilation variables:
  go test ./config ./models ./utils/intID
Source fixture config is supplied through OPTIRAY_TEST_CONFIG; no personal data is included.

Keenetic policy scope: routing groups respect interfaces enabled in each connection policy.
Policy state is refreshed every 5 seconds and on netfilter hooks; RCI failures pause
non-blocking interception. NAT applies only to the mark owned by the routing group.
Linux target regression tests (run the resulting executable on that target):
  go test -mod=vendor -tags testing -c -o policy-guard.test ./utils/netfilterTools

Both targets use static binaries without CGO. Each IPK contains only its native
binary. Generic uname mips is accepted only with the mipsel-3.4_kn Entware ABI.
