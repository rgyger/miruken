package httpsrv

import (
	"github.com/miruken-go/miruken"
	"github.com/miruken-go/miruken/api/http"
)

// Installer configures http server support
type Installer struct {}

func (i *Installer) DependsOn() []miruken.Feature {
	return []miruken.Feature{http.Feature()}
}

func (i *Installer) Install(setup *miruken.SetupBuilder) error {
	if setup.Tag(&featureTag) {
		setup.Specs(
			&ApiHandler{},
			&StatusCodeMapper{})
	}
	return nil
}

// Feature configures http server support
func Feature(config ...func(*Installer)) miruken.Feature {
	installer := &Installer{}
	for _, configure := range config {
		if configure != nil {
			configure(installer)
		}
	}
	return installer
}

var featureTag byte