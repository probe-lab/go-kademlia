package libp2p

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-msgio/pbio"
	"github.com/multiformats/go-multiaddr"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/probe-lab/go-kademlia/event"
	"github.com/probe-lab/go-kademlia/kad"
	"github.com/probe-lab/go-kademlia/key"
	"github.com/probe-lab/go-kademlia/network/address"
	"github.com/probe-lab/go-kademlia/network/endpoint"
	"github.com/probe-lab/go-kademlia/util"
)

type DialReportFn func(context.Context, bool)

// TODO: Use sync.Pool to reuse buffers https://pkg.go.dev/sync#Pool

type Libp2pEndpoint struct {
	ctx   context.Context
	host  host.Host
	sched event.Scheduler

	// peer filters to be applied before adding peer to peerstore

	writers sync.Pool
	readers sync.Pool
}

var (
	_ endpoint.NetworkedEndpoint[key.Key256, multiaddr.Multiaddr] = (*Libp2pEndpoint)(nil)
	_ endpoint.ServerEndpoint[key.Key256, multiaddr.Multiaddr]    = (*Libp2pEndpoint)(nil)
)

func NewLibp2pEndpoint(ctx context.Context, host host.Host,
	sched event.Scheduler,
) *Libp2pEndpoint {
	return &Libp2pEndpoint{
		ctx:     ctx,
		host:    host,
		sched:   sched,
		writers: sync.Pool{},
		readers: sync.Pool{},
	}
}

func getPeerID(id kad.NodeID[key.Key256]) (*PeerID, error) {
	if p, ok := id.(*PeerID); ok {
		return p, nil
	}
	return nil, endpoint.ErrInvalidPeer
}

func (e *Libp2pEndpoint) AsyncDialAndReport(ctx context.Context,
	id kad.NodeID[key.Key256], reportFn DialReportFn,
) error {
	p, err := getPeerID(id)
	if err != nil {
		return err
	}
	if e.host.Network().Connectedness(p.ID) == network.Connected {
		// if peer is already connected, no need to dial
		if reportFn != nil {
			reportFn(ctx, true)
		}
		return nil
	}
	go func() {
		ctx, span := util.StartSpan(ctx, "Libp2pEndpoint.AsyncDialAndReport",
			trace.WithAttributes(attribute.String("PeerID", p.String())))
		defer span.End()

		success := true
		if err := e.DialPeer(ctx, p); err != nil {
			span.AddEvent("dial failed", trace.WithAttributes(
				attribute.String("Error", err.Error()),
			))
			success = false
		} else {
			span.AddEvent("dial successful")
		}

		if reportFn != nil {
			// report dial result where it is needed
			e.sched.EnqueueAction(ctx, event.BasicAction(func(ctx context.Context) {
				reportFn(ctx, success)
			}))
		}
	}()
	return nil
}

func (e *Libp2pEndpoint) DialPeer(ctx context.Context, id kad.NodeID[key.Key256]) error {
	p, err := getPeerID(id)
	if err != nil {
		return err
	}

	_, span := util.StartSpan(ctx, "Libp2pEndpoint.DialPeer", trace.WithAttributes(
		attribute.String("PeerID", p.String()),
	))
	defer span.End()

	if e.host.Network().Connectedness(p.ID) == network.Connected {
		span.AddEvent("Already connected")
		return nil
	}

	pi := peer.AddrInfo{ID: p.ID}
	if err := e.host.Connect(ctx, pi); err != nil {
		span.AddEvent("Connection failed", trace.WithAttributes(
			attribute.String("Error", err.Error()),
		))
		return err
	}
	span.AddEvent("Connection successful")
	return nil
}

func (e *Libp2pEndpoint) MaybeAddToPeerstore(ctx context.Context,
	id kad.NodeInfo[key.Key256, multiaddr.Multiaddr], ttl time.Duration,
) error {
	_, span := util.StartSpan(ctx, "Libp2pEndpoint.MaybeAddToPeerstore",
		trace.WithAttributes(attribute.String("PeerID", id.ID().String())))
	defer span.End()

	ai, ok := id.(*AddrInfo)
	if !ok {
		return endpoint.ErrInvalidPeer
	}

	// Don't add addresses for self or our connected peers. We have better ones.
	if ai.PeerID().ID == e.host.ID() ||
		e.host.Network().Connectedness(ai.PeerID().ID) == network.Connected {
		return nil
	}
	e.host.Peerstore().AddAddrs(ai.PeerID().ID, ai.Addrs, ttl)
	return nil
}

func (e *Libp2pEndpoint) SendRequestHandleResponse(ctx context.Context,
	protoID address.ProtocolID, n kad.NodeID[key.Key256], req kad.Message,
	resp kad.Message, timeout time.Duration,
	responseHandlerFn endpoint.ResponseHandlerFn[key.Key256, multiaddr.Multiaddr],
) error {
	_, span := util.StartSpan(ctx,
		"Libp2pEndpoint.SendRequestHandleResponse", trace.WithAttributes(
			attribute.String("PeerID", n.String()),
		))
	defer span.End()

	protoResp, ok := resp.(ProtoKadResponseMessage[key.Key256, multiaddr.Multiaddr])
	if !ok {
		span.RecordError(ErrRequireProtoKadResponse)
		return ErrRequireProtoKadResponse
	}

	protoReq, ok := req.(ProtoKadMessage)
	if !ok {
		span.RecordError(ErrRequireProtoKadMessage)
		return ErrRequireProtoKadMessage
	}

	p, ok := n.(*PeerID)
	if !ok {
		span.RecordError(ErrRequirePeerID)
		return ErrRequirePeerID
	}

	if len(e.host.Peerstore().Addrs(p.ID)) == 0 {
		span.RecordError(endpoint.ErrUnknownPeer)
		return endpoint.ErrUnknownPeer
	}

	if responseHandlerFn == nil {
		span.RecordError(endpoint.ErrNilResponseHandler)
		return endpoint.ErrNilResponseHandler
	}

	go func() {
		ctx, span := util.StartSpan(e.ctx,
			"Libp2pEndpoint.SendRequestHandleResponse libp2p go routine",
			trace.WithAttributes(
				attribute.String("PeerID", n.String()),
			))
		defer span.End()
		var cancel context.CancelFunc
		if timeout > 0 {
			ctx, cancel = e.sched.Clock().WithTimeout(ctx, timeout)
		} else {
			ctx, cancel = context.WithCancel(ctx)
		}
		defer cancel()

		var err error

		var s network.Stream
		s, err = e.host.NewStream(ctx, p.ID, protocol.ID(protoID))
		if err != nil {
			span.RecordError(err, trace.WithAttributes(attribute.String("where", "stream creation")))
			e.sched.EnqueueAction(ctx, event.BasicAction(func(ctx context.Context) {
				responseHandlerFn(ctx, nil, err)
			}))
			return
		}
		defer s.Close()

		err = WriteMsg(s, protoReq)
		if err != nil {
			span.RecordError(err, trace.WithAttributes(attribute.String("where", "write message")))
			e.sched.EnqueueAction(ctx, event.BasicAction(func(ctx context.Context) {
				responseHandlerFn(ctx, nil, err)
			}))
			return
		}

		var timeoutEvent event.PlannedAction
		// handle timeout

		if timeout != 0 {
			timeoutEvent = event.ScheduleActionIn(ctx, e.sched, timeout,
				event.BasicAction(func(ctx context.Context) {
					cancel()
					responseHandlerFn(ctx, nil, endpoint.ErrTimeout)
				}))
		}

		err = ReadMsg(s, protoResp)
		if timeout != 0 {
			// remove timeout if not too late
			if !e.sched.RemovePlannedAction(ctx, timeoutEvent) {
				span.RecordError(endpoint.ErrResponseReceivedAfterTimeout)
				// don't run responseHandlerFn if timeout was already triggered
				return
			}
		}
		if err != nil {
			span.RecordError(err, trace.WithAttributes(attribute.String("where", "read message")))
			e.sched.EnqueueAction(ctx, event.BasicAction(func(ctx context.Context) {
				responseHandlerFn(ctx, protoResp, err)
			}))
			return
		}

		span.AddEvent("response received")
		e.sched.EnqueueAction(ctx, event.BasicAction(func(ctx context.Context) {
			responseHandlerFn(ctx, protoResp, err)
		}))
	}()
	return nil
}

func (e *Libp2pEndpoint) Connectedness(id kad.NodeID[key.Key256]) (endpoint.Connectedness, error) {
	p, err := getPeerID(id)
	if err != nil {
		return endpoint.NotConnected, err
	}

	c := e.host.Network().Connectedness(p.ID)
	switch c {
	case network.NotConnected:
		return endpoint.NotConnected, nil
	case network.Connected:
		return endpoint.Connected, nil
	case network.CanConnect:
		return endpoint.CanConnect, nil
	case network.CannotConnect:
		return endpoint.CannotConnect, nil
	default:
		panic(fmt.Sprintf("unexpected libp2p connectedness value: %v", c))
	}
}

func (e *Libp2pEndpoint) PeerInfo(id kad.NodeID[key.Key256]) (peer.AddrInfo, error) {
	p, err := getPeerID(id)
	if err != nil {
		return peer.AddrInfo{}, err
	}
	return e.host.Peerstore().PeerInfo(p.ID), nil
}

func (e *Libp2pEndpoint) Key() key.Key256 {
	return PeerID{ID: e.host.ID()}.Key()
}

func (e *Libp2pEndpoint) NetworkAddress(n kad.NodeID[key.Key256]) (kad.NodeInfo[key.Key256, multiaddr.Multiaddr], error) {
	ai, err := e.PeerInfo(n)
	if err != nil {
		return nil, err
	}
	return NewAddrInfo(ai), nil
}

func (e *Libp2pEndpoint) AddRequestHandler(protoID address.ProtocolID,
	req kad.Message, reqHandler endpoint.RequestHandlerFn[key.Key256],
) error {
	protoReq, ok := req.(ProtoKadMessage)
	if !ok {
		return ErrRequireProtoKadMessage
	}
	if reqHandler == nil {
		return endpoint.ErrNilRequestHandler
	}
	// when a new request comes in, we need to queue it
	streamHandler := func(s network.Stream) {
		e.sched.EnqueueAction(e.ctx, event.BasicAction(func(ctx context.Context) {
			ctx, span := util.StartSpan(ctx, "Libp2pEndpoint.AddRequestHandler",
				trace.WithAttributes(
					attribute.String("PeerID", s.Conn().RemotePeer().String()),
				))
			defer span.End()
			defer s.Close()

			// create a protobuf reader and writer
			r := pbio.NewDelimitedReader(s, network.MessageSizeMax)
			w := pbio.NewDelimitedWriter(s)

			for {
				// read a message from the stream
				err := r.ReadMsg(protoReq)
				if err != nil {
					if err == io.EOF {
						// stream EOF, all done
						return
					}
					span.RecordError(err)
					return
				}

				requester := NewAddrInfo(
					e.host.Peerstore().PeerInfo(s.Conn().RemotePeer()),
				)
				resp, err := reqHandler(ctx, requester, req)
				if err != nil {
					span.RecordError(err)
					return
				}

				protoResp, ok := resp.(ProtoKadMessage)
				if !ok {
					err = errors.New("Libp2pEndpoint requires ProtoKadMessage")
					span.RecordError(err)
					return
				}

				// write the response to the stream
				err = w.WriteMsg(protoResp)
				if err != nil {
					span.RecordError(err)
					return
				}
			}
		}))
	}
	e.host.SetStreamHandler(protocol.ID(protoID), streamHandler)
	return nil
}

func (e *Libp2pEndpoint) RemoveRequestHandler(protoID address.ProtocolID) {
	e.host.RemoveStreamHandler(protocol.ID(protoID))
}
