// Code generated by https://github.com/Miruken-Go/miruken/tools/cmd/miruken; DO NOT EDIT.

package test

import "github.com/miruken-go/miruken"

var TestFeature = miruken.InstallFeature(func(setup *miruken.SetupBuilder) error {
	setup.RegisterHandlers(
		&PlayerMapper{},
		&TypeIdMapper{},
	)
	return nil
})
