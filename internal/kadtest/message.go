package kadtest

import (
	"github.com/plprobelab/go-kademlia/kad"
	"github.com/plprobelab/go-kademlia/key"
)

type Request[K kad.Key[K], N kad.NodeID[K]] struct {
	target K
	id     string
}

func NewRequest[K kad.Key[K], N kad.NodeID[K]](id string, target K) *Request[K, N] {
	return &Request[K, N]{
		target: target,
		id:     id,
	}
}

func (r *Request[K, N]) Target() K {
	return r.target
}

func (r *Request[K, N]) ID() string {
	return r.id
}

func (r *Request[K, N]) EmptyResponse() kad.Response[K, N] {
	return &Response[K, N]{}
}

type Response[K kad.Key[K], N kad.NodeID[K]] struct {
	id     string
	closer []N
}

func NewResponse[K kad.Key[K], N kad.NodeID[K]](id string, closer []N) *Response[K, N] {
	return &Response[K, N]{
		id:     id,
		closer: closer,
	}
}

func (r *Response[K, N]) ID() string {
	return r.id
}

func (r *Response[K, N]) CloserNodes() []N {
	return r.closer
}

type (
	// Request8 is a Request message that uses key.Key8
	Request8 = Request[key.Key8, ID[key.Key8]]

	// Response8 is a Response message that uses key.Key8
	Response8 = Response[key.Key8, ID[key.Key8]]

	// Request8 is a Request message that uses key.Key256
	Request256 = Request[key.Key256, ID[key.Key256]]

	// Response256 is a Response message that uses key.Key256
	Response256 = Response[key.Key256, ID[key.Key256]]
)

var (
	_ kad.Request[key.Key8, ID[key.Key8]]  = (*Request8)(nil)
	_ kad.Response[key.Key8, ID[key.Key8]] = (*Response8)(nil)
)

var (
	_ kad.Request[key.Key256, ID[key.Key256]]  = (*Request256)(nil)
	_ kad.Response[key.Key256, ID[key.Key256]] = (*Response256)(nil)
)
