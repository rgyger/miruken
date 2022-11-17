package json

import (
	"encoding/json"
	"fmt"
	"github.com/miruken-go/miruken"
	"io"
	"reflect"
)

// StdMapper formats to and from json using encoding/json.
type (
	StdMapper struct{}

	StdOptions struct {
		Prefix            string
		Indent            string
		TypeFieldHandling miruken.Option[TypeFieldHandling]
	}
)

// StdMapper

func (m *StdMapper) ToJson(
	_*struct{
		miruken.Maps
		miruken.Format `as:"application/json"`
	  }, maps *miruken.Maps,
	_*struct{
	    miruken.Optional
	    miruken.FromOptions
	  }, options StdOptions,
	ctx miruken.HandleContext,
) (js string, err error) {
	var data []byte
	src := maps.Source()
	if options.TypeFieldHandling == miruken.SetOption(TypeFieldHandlingRoot) {
		data, err = json.Marshal(typeEncodeContainer{
			src,
			&options,
			ctx.Composer(),
		})
	} else if prefix, indent := options.Prefix, options.Indent; len(prefix) > 0 || len(indent) > 0 {
		data, err = json.MarshalIndent(src, prefix, indent)
	} else {
		data, err = json.Marshal(src)
	}
	return string(data), err
}

func (m *StdMapper) ToJsonStream(
	_*struct{
	    miruken.Maps
		miruken.Format `as:"application/json"`
	  }, maps *miruken.Maps,
	_*struct{
	    miruken.Optional
	    miruken.FromOptions
	  }, options StdOptions,
) (stream io.Writer, err error) {
	if writer, ok := maps.Target().(*io.Writer); ok && !miruken.IsNil(writer) {
		enc := json.NewEncoder(*writer)
		if prefix, indent := options.Prefix, options.Indent; len(prefix) > 0 || len(indent) > 0 {
			enc.SetIndent(prefix, indent)
		}
		err    = enc.Encode(maps.Source())
		stream = *writer
	}
	return stream, err
}

func (m *StdMapper) FromJson(
	_*struct{
	    miruken.Maps
		miruken.Format `as:"application/json"`
	  }, jsonString string,
	maps *miruken.Maps,
) (any, error) {
	target := maps.Target()
	err    := json.Unmarshal([]byte(jsonString), target)
	return target, err
}

func (m *StdMapper) FromJsonStream(
	_*struct{
	    miruken.Maps
		miruken.Format `as:"application/json"`
	  }, stream io.Reader,
	maps *miruken.Maps,
) (any, error) {
	target := maps.Target()
	dec    := json.NewDecoder(stream)
	err    := dec.Decode(target)
	return target, err
}

type (
	typeEncodeContainer struct {
		v         any
		options  *StdOptions
		composer  miruken.Handler
	}
)

// typeEncodeContainer

func (c typeEncodeContainer) MarshalJSON() ([]byte, error) {
	v   := c.v
	if byt, err := json.Marshal(v); err != nil {
		return nil, err
	} else {
		if typ := reflect.TypeOf(v); typ != nil && typ.Kind() == reflect.Struct {
			typInfo, _, err := miruken.Map[TypeFieldInfo](c.composer, v, "type:json")
			if err != nil {
				return nil, fmt.Errorf("no type info \"%v\": %w", typ, err)
			}
			typeId := []byte(fmt.Sprintf("\"%v\":\"%v\",", typInfo.Name, typInfo.Value))
			byt = append(byt, typeId...)
			copy(byt[len(typeId)+1:], byt[1:])
			copy(byt[1:], typeId)
		}
		return byt, nil
	}
}