package stdjson

import (
	"bytes"
	"encoding/json"
	"github.com/Rican7/conjson"
	"github.com/Rican7/conjson/transform"
	"github.com/miruken-go/miruken"
	"github.com/miruken-go/miruken/api"
	"github.com/miruken-go/miruken/args"
	"github.com/miruken-go/miruken/internal"
	"github.com/miruken-go/miruken/maps"
	"io"
)

type (
	// Options provide options for controlling json encoding.
	Options struct {
		Prefix       string
		Indent       string
		EscapeHTML   miruken.Option[bool]
		Transformers []transform.Transformer
	}

	// Mapper formats to and from json using encoding/json.
	Mapper struct{}
)


var (
	// CamelCase directs the json encoding of keys to use camelcase notation.
	CamelCase = miruken.Options(Options{
		Transformers: []transform.Transformer{
			transform.OnlyForDirection(
				transform.Marshal,
				transform.CamelCaseKeys(false)),
		},
	})
)


func (m *Mapper) ToJson(
	_*struct{
		maps.Format `to:"application/json"`
	  }, it *maps.It,
	_*struct{
		args.Optional
		args.FromOptions
	  }, options Options,
	_*struct{
		args.Optional
		args.FromOptions
	  }, apiOptions api.Options,
	ctx miruken.HandleContext,
) (json any, err error) {
	switch t := it.Target().(type) {
	case *[]byte:
		return marshal(it, t, &options, &apiOptions, ctx.Composer)
	case *io.Writer:
		if internal.IsNil(*t) {
			*t = new(bytes.Buffer)
		}
		if err = encode(it, *t, &options, &apiOptions, ctx.Composer); err == nil {
			json = *t
		}
	}
	return
}

func (m *Mapper) FromBytes(
	_*struct{
		maps.Format `from:"application/json"`
	  }, byt []byte,
	_*struct{
		args.Optional
		args.FromOptions
	  }, options Options,
	_*struct{
		args.Optional
		args.FromOptions
	  }, apiOptions api.Options,
	maps *maps.It,
	ctx  miruken.HandleContext,
) (any, error) {
	return unmarshal(maps, byt, &options, &apiOptions, ctx.Composer)
}

func (m *Mapper) FromReader(
	_*struct{
		maps.Format `from:"application/json"`
	  }, reader io.Reader,
	_*struct{
		args.Optional
		args.FromOptions
	  }, options Options,
	_*struct{
		args.Optional
		args.FromOptions
	  }, apiOptions api.Options,
	maps *maps.It,
	ctx  miruken.HandleContext,
) (any, error) {
	return decode(maps, reader, &options, &apiOptions, ctx.Composer)
}

func marshal(
	it         *maps.It,
	byt        *[]byte,
	options    *Options,
	apiOptions *api.Options,
	composer   miruken.Handler,
) ([]byte, error) {
	src := it.Source()
	it.TargetForWrite()
	if apiOptions.Polymorphism == miruken.Set(api.PolymorphismRoot) {
		src = &typeContainer{
			v:        src,
			typInfo:  apiOptions.TypeInfoFormat,
			trans:    options.Transformers,
			composer: composer,
		}
	} else if trans := options.Transformers; len(trans) > 0 {
		src = &transformer{src, trans}
	}
	var err error
	if prefix, indent := options.Prefix, options.Indent; len(prefix) > 0 || len(indent) > 0 {
		*byt, err = json.MarshalIndent(src, prefix, indent)
	} else {
		*byt, err = json.Marshal(src)
	}
	return *byt, err
}

func unmarshal(
	maps       *maps.It,
	byt        []byte,
	options    *Options,
	apiOptions *api.Options,
	composer   miruken.Handler,
) (target any, err error) {
	target = maps.TargetForWrite()
	if apiOptions.Polymorphism == miruken.Set(api.PolymorphismRoot) {
		tc := typeContainer{
			v:        target,
			trans:    options.Transformers,
			composer: composer,
		}
		err = json.Unmarshal(byt, &tc)
	} else {
		if trans := options.Transformers; len(trans) > 0 {
			t := transformer{target, trans}
			target = &t
		}
		err = json.Unmarshal(byt, target)
	}
	return
}

func encode(
	it         *maps.It,
	writer     io.Writer,
	options    *Options,
	apiOptions *api.Options,
	composer   miruken.Handler,
) error {
	it.TargetForWrite()
	enc := json.NewEncoder(writer)
	if prefix, indent := options.Prefix, options.Indent; len(prefix) > 0 || len(indent) > 0 {
		enc.SetIndent(prefix, indent)
	}
	if escapeHTML := options.EscapeHTML; escapeHTML.Set() {
		enc.SetEscapeHTML(escapeHTML.Value())
	}
	src := it.Source()
	if apiOptions.Polymorphism == miruken.Set(api.PolymorphismRoot) {
		src = &typeContainer{
			v:        src,
			typInfo:  apiOptions.TypeInfoFormat,
			trans:    options.Transformers,
			composer: composer,
		}
	} else if trans := options.Transformers; len(trans) > 0 {
		src = &transformer{src, trans}
	}
	return enc.Encode(src)
}

func decode(
	it         *maps.It,
	reader     io.Reader,
	options    *Options,
	apiOptions *api.Options,
	composer   miruken.Handler,
) (target any, err error) {
	target = it.TargetForWrite()
	dec := json.NewDecoder(reader)
	if apiOptions.Polymorphism == miruken.Set(api.PolymorphismRoot) {
		tc := typeContainer{
			v:        target,
			trans:    options.Transformers,
			composer: composer,
		}
		err = dec.Decode(&tc)
	} else {
		if trans := options.Transformers; len(trans) > 0 {
			t := transformer{target, trans}
			target = &t
		}
		err = dec.Decode(target)
	}
	return
}


// transformer applies transformations to json serialization.
type transformer struct {
	v     any
	trans []transform.Transformer
}

func (t *transformer) MarshalJSON() ([]byte, error) {
	conventions := conjson.NewMarshaler(t.v, t.trans...)
	return json.Marshal(conventions)
}

func (t *transformer) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, conjson.NewUnmarshaler(t.v, t.trans...))
}
