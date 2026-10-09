package constant

import (
	"time"

	"optirout/models"
)

var DefaultAppConfig = models.AppConfig{
	DNSProxy: models.AppConfigDNSProxy{
		Host:            models.AppConfigDNSProxyServer{Address: "[::]", Port: 3553},
		Upstream:        models.AppConfigDNSProxyServer{Address: "127.0.0.1", Port: 53},
		DisableRemap53:  false,
		DisableFakePTR:  false,
		DisableDropAAAA: false,
		MaxIdleConns:    10,
		MaxConcurrent:   100,
		Timeout:         5000 * time.Millisecond,
	},
	HTTPWeb: models.AppConfigHTTPWeb{
		Enabled: false,
		Auth: models.AppConfigAuth{
			Enabled: false,
		},
		Host: models.AppConfigHTTPWebServer{
			Address: "[::]",
			Port:    8080,
		},
		Skin: "default",
	},
	Netfilter: models.AppConfigNetfilter{
		IPTables: models.AppConfigIPTables{
			ChainPrefix: "OR_",
		},
		IPSet: models.AppConfigIPSet{
			TablePrefix:   "or_",
			AdditionalTTL: 1 * time.Hour,
		},
		DisableIPv4:         false,
		DisableIPv6:         false,
		StartMarkTableIndex: 0x4D616769, // Magi
	},
	Link:              []string{"br0"},
	ShowAllInterfaces: false,
	LogLevel:          "info",
}

var (
	Version = "unattached"
)
