package stdjson

import (
	"encoding/json"
	"github.com/miruken-go/miruken"
	"github.com/miruken-go/miruken/api"
	"github.com/miruken-go/miruken/maps"
	"io"
)

// MessageSurrogate is a json standard surrogate for api.Message.
type MessageSurrogate struct {
	Payload json.RawMessage `json:"payload"`
}


func (m *SurrogateMapper) EncodeMessage(
	_*struct{
		maps.Format `to:"application/json"`
	  }, msg api.Message,
	it *maps.It,
	ctx miruken.HandleContext,
) (io.Writer, error) {
	if writer, ok := it.Target().(*io.Writer); ok {
		var sur MessageSurrogate
		if payload := msg.Payload; payload != nil {
			pb, _, _, err := maps.Out[[]byte](ctx.Composer, msg.Payload, api.ToJson)
			if err != nil {
				return nil, err
			}
			sur.Payload = pb
		}
		enc := json.NewEncoder(*writer)
		if err := enc.Encode(sur); err == nil {
			it.TargetForWrite()
			return *writer, err
		}
	}
	return nil, nil
}

func (m *SurrogateMapper) DecodeMessage(
	_*struct{
		maps.It
		maps.Format `from:"application/json"`
	  }, reader io.Reader,
	it *maps.It,
	ctx miruken.HandleContext,
) (msg api.Message, err error) {
	if mp, ok := it.Target().(*api.Message); ok {
		var sur MessageSurrogate
		dec := json.NewDecoder(reader)
		if err = dec.Decode(&sur); err != nil {
			return
		}
		if payload := sur.Payload; payload != nil {
			var late api.Late
			composer := ctx.Composer
			late, _, _, err = maps.Out[api.Late](composer, []byte(payload), api.FromJson)
			if sur, ok := late.Value.(api.Surrogate); ok {
				late.Value, err = sur.Original(composer)
			}
			it.TargetForWrite()
			mp.Payload = late.Value
			msg = *mp
		}
	}
	return
}