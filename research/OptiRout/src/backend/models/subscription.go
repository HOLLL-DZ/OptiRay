package models

import "optirout/utils/intID"

type SubscriptionRule struct {
	ID     intID.ID `json:"id"`
	Rule   string   `json:"rule"`
	Type   string   `json:"type"`
	Enable bool     `json:"enable"`
}

type Subscription struct {
	ID         intID.ID            `json:"id"`
	Name       string              `json:"name"`
	Interface  string              `json:"interface"`
	Enable     bool                `json:"enable"`
	URL        string              `json:"url"`
	Interval   uint32              `json:"interval"`
	LastUpdate uint32              `json:"last_update"`
	LastCheck  uint32              `json:"-"`
	Rules      []*SubscriptionRule `json:"rules"`
}
