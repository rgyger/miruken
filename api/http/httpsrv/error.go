package httpsrv

import (
	"encoding/json"
	"github.com/miruken-go/miruken"
	"github.com/miruken-go/miruken/api"
	"github.com/miruken-go/miruken/validate"
	"net/http"
)

type (
	StatusCodeMapper struct {}
)

func (s *StatusCodeMapper) NotHandled(
	_*struct{
		miruken.Maps
		miruken.Format `to:"http:status-code"`
	  }, nhe *miruken.NotHandledError,
) int {
	if _, ok := nhe.Callback.(*miruken.Maps); ok {
		return http.StatusUnsupportedMediaType
	}
	return http.StatusInternalServerError
}

func (s *StatusCodeMapper) InvalidTypeId(
	_*struct{
		miruken.Maps
		miruken.Format `to:"http:status-code"`
	}, _ *api.InvalidTypeIdError,
) int {
	return http.StatusBadRequest
}

func (s *StatusCodeMapper) Validation(
	_*struct{
		miruken.Maps
		miruken.Format `to:"http:status-code"`
	  }, _ *validate.Outcome,
) int {
	return http.StatusUnprocessableEntity
}

func (s *StatusCodeMapper) JsonSyntax(
	_*struct{
		miruken.Maps
		miruken.Format `to:"http:status-code"`
	}, _ *json.SyntaxError,
) int {
	return http.StatusBadRequest
}