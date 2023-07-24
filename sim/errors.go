package sim

import "errors"

var (
	ErrNotNetworkedEndpoint = errors.New("endpoint is not a NetworkedEndpoint")
	ErrUnknownMessageFormat = errors.New("unknown message format")
	ErrInvalidResponseType  = errors.New("invalid response type, expected MinKadResponseMessage")
)
