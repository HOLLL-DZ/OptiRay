package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func (a *App) uiAsset(name string) ([]byte, error) {
	switch name {
	case "page.html", "style.css", "app.js":
	default:
		return nil, fmt.Errorf("unknown UI asset")
	}
	if a.uiDir != "" {
		return os.ReadFile(filepath.Join(a.uiDir, name))
	}
	return assets.ReadFile("assets/" + name)
}
