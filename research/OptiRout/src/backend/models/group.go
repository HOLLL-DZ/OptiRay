package models

import (
	"optirout/utils/intID"
)

type Group struct {
	ID        intID.ID `json:"id"`
	Name      string   `json:"name"`
	Color     string   `json:"color"`
	Interface string   `json:"interface"`
	Enable    bool     `json:"enable"`
	Rules     []*Rule  `json:"rules"`
}
