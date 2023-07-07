package endpoint

import "errors"

var (
	ErrCannotConnect                = errors.New("cannot connect")
	ErrUnknownPeer                  = errors.New("unknown peer")
	ErrInvalidPeer                  = errors.New("invalid peer")
	ErrTimeout                      = errors.New("request timeout")
	ErrNilRequestHandler            = errors.New("nil request handler")
	ErrNilResponseHandler           = errors.New("nil response handler")
	ErrResponseReceivedAfterTimeout = errors.New("response received after timeout")
)
