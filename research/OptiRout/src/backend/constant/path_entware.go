//go:build entware

package constant

import "os"

func entwarePath(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

var (
	AppConfigDir = entwarePath("OPTIRAY_ROUTING_CONFIG_DIR", "/opt/etc/optiray/routing")
	AppStateDir  = entwarePath("OPTIRAY_ROUTING_STATE_DIR", AppConfigDir)
	PIDPath      = entwarePath("OPTIRAY_ROUTING_PID", "/opt/var/run/optirout.pid")
	SockPath     = entwarePath("OPTIRAY_ROUTING_SOCKET", "/opt/var/run/optirout.sock")
)

const (
	AppShareDir = "/opt/usr/share/optirout"
	PasswdFile  = "/opt/etc/passwd"
	ShadowFile  = "/opt/etc/shadow"
)
