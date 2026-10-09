package config

import "time"

type App struct {
	HTTPWeb           *HTTPWeb   `json:"httpWeb"`
	DNSProxy          *DNSProxy  `json:"dnsProxy"`
	Netfilter         *Netfilter `json:"netfilter"`
	Link              *[]string  `json:"link"`
	ShowAllInterfaces *bool      `json:"showAllInterfaces"`
	LogLevel          *string    `json:"logLevel"`
}

type HTTPWeb struct {
	Enabled *bool          `json:"enabled"`
	Auth    *Auth          `json:"auth"`
	Host    *HTTPWebServer `json:"host"`
	Skin    *string        `json:"skin"`
}

type Auth struct {
	Enabled *bool `json:"enabled"`
}

type HTTPWebServer struct {
	Address *string `json:"address"`
	Port    *uint16 `json:"port"`
}

type DNSProxy struct {
	Host            *DNSProxyServer `json:"host"`
	Upstream        *DNSProxyServer `json:"upstream"`
	DisableRemap53  *bool           `json:"disableRemap53"`
	DisableFakePTR  *bool           `json:"disableFakePTR"`
	DisableDropAAAA *bool           `json:"disableDropAAAA"`
	MaxIdleConns    *uint           `json:"maxIdleConns"`
	MaxConcurrent   *uint           `json:"maxConcurrent"`
	Timeout         *time.Duration  `json:"timeout"`
}

type DNSProxyServer struct {
	Address *string `json:"address"`
	Port    *uint16 `json:"port"`
}

type Netfilter struct {
	IPTables            *IPTables `json:"iptables"`
	IPSet               *IPSet    `json:"ipset"`
	DisableIPv4         *bool     `json:"disableIPv4"`
	DisableIPv6         *bool     `json:"disableIPv6"`
	StartMarkTableIndex *uint32   `json:"startMarkTableIndex"`
}

type IPTables struct {
	ChainPrefix *string `json:"chainPrefix"`
}

type IPSet struct {
	TablePrefix   *string        `json:"tablePrefix"`
	AdditionalTTL *time.Duration `json:"additionalTTL"`
}
