package basicserver

import "fmt"

var (
	ErrNotNetworkedEndpoint = fmt.Errorf("endpoint is not a NetworkedEndpoint")
	ErrUnknownMessageFormat = fmt.Errorf("unknown message format")
	ErrIpfsV1InvalidPeerID  = fmt.Errorf("IpfsV1 Message contains invalid peer.ID")
	ErrIpfsV1InvalidRequest = fmt.Errorf("IpfsV1 Message unknown request type")
	ErrSimMessageNilTarget  = fmt.Errorf("SimMessage target is nil")
)
