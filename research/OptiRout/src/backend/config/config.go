package config

import "optirout/models"

type Config struct {
	ConfigVersion string                  `json:"configVersion"`
	App           *App                    `json:"app"`
	Groups        *[]*models.Group        `json:"groups"`
	Subscriptions *[]*models.Subscription `json:"subscriptions"`
}
