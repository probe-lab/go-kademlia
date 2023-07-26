package basicserver

import "errors"

var (
	ErrNotNetworkedEndpoint = errors.New("endpoint is not a NetworkedEndpoint")
	ErrUnknownMessageFormat = errors.New("unknown message format")
	ErrIpfsV1InvalidPeerID  = errors.New("IpfsV1 Message contains invalid peer.ID")
	ErrIpfsV1InvalidRequest = errors.New("IpfsV1 Message unknown request type")
	ErrSimMessageNilTarget  = errors.New("SimMessage target is nil")
)
