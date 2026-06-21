package protocol

import (
	"encoding/json"
	"fmt"
)

// Encode marshals a message struct to JSON. The struct must set its MsgType.
func Encode(msg any) ([]byte, error) {
	b, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("encoding %T: %w", msg, err)
	}
	return b, nil
}

// Decode parses a Basic Station message, dispatching on its msgtype to the
// matching struct. The returned value is one of *Version, *RouterConfig, *Jreq,
// *Updf, *Dnmsg, or *Dntxed. An unknown msgtype returns an *Envelope and an
// error wrapping ErrUnknownType so callers can choose to ignore it.
func Decode(data []byte) (any, error) {
	var env Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("peeking msgtype: %w", err)
	}

	var out any
	switch env.MsgType {
	case TypeVersion:
		out = &Version{}
	case TypeRouterConfig:
		out = &RouterConfig{}
	case TypeJreq:
		out = &Jreq{}
	case TypeUpdf:
		out = &Updf{}
	case TypeDnmsg:
		out = &Dnmsg{}
	case TypeDntxed:
		out = &Dntxed{}
	default:
		return &env, fmt.Errorf("%w: %q", ErrUnknownType, env.MsgType)
	}

	if err := json.Unmarshal(data, out); err != nil {
		return nil, fmt.Errorf("decoding %s: %w", env.MsgType, err)
	}
	return out, nil
}

// ErrUnknownType is wrapped by Decode for unrecognized msgtypes.
var ErrUnknownType = errUnknownType{}

type errUnknownType struct{}

func (errUnknownType) Error() string { return "unknown msgtype" }
