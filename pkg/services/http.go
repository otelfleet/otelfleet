package services

import (
	"github.com/gorilla/mux"
	"github.com/grafana/dskit/services"
)

type HTTPExtension interface {
	services.Service
	// TODO : can probably have some params like configure auth/log middleware
	ConfigureHTTP(*mux.Router)
}
