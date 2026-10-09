//go:build !entware && !openwrt

package constant

const (
	AppConfigDir = "/etc/optirout"
	AppShareDir  = "/usr/share/optirout"
	AppStateDir  = "/var/lib/optirout"
	PIDPath      = "/var/run/optirout.pid"
	SockPath     = "/var/run/optirout.sock"
	PasswdFile   = "/etc/passwd"
	ShadowFile   = "/etc/shadow"
)
