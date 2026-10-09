package api

import (
	"github.com/rs/zerolog/log"
	"net/http"
	"optirout/app"
)

// OptiRay is the only WebUI. The separate Unix socket API remains enabled.
func SetupHTTP(a app.Main, errChan chan error) (*http.Server, error) {
	log.Info().Msg("OptiRout: standalone HTTP WebUI is not included")
	return nil, nil
}
