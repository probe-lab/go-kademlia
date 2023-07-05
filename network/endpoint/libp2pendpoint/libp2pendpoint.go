package libp2pendpoint

import (
	"context"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-msgio/pbio"
	ba "github.com/plprobelab/go-kademlia/events/action/basicaction"
	"github.com/plprobelab/go-kademlia/events/planner"
	"github.com/plprobelab/go-kademlia/events/scheduler"
	"github.com/plprobelab/go-kademlia/key"
	"github.com/plprobelab/go-kademlia/network/address"
	"github.com/plprobelab/go-kademlia/network/address/addrinfo"
	"github.com/plprobelab/go-kademlia/network/address/peerid"
	"github.com/plprobelab/go-kademlia/network/endpoint"
	"github.com/plprobelab/go-kademlia/network/message"
	"github.com/plprobelab/go-kademlia/util"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type DialReportFn func(context.Context, bool)

// TODO: Use sync.Pool to reuse buffers https://pkg.go.dev/sync#Pool

type Libp2pEndpoint struct {
	ctx   context.Context
	host  host.Host
	sched scheduler.Scheduler

	// peer filters to be applied before adding peer to peerstore

	writers sync.Pool
	readers sync.Pool
}

var _ endpoint.NetworkedEndpoint = (*Libp2pEndpoint)(nil)
var _ endpoint.ServerEndpoint = (*Libp2pEndpoint)(nil)

func NewLibp2pEndpoint(ctx context.Context, host host.Host,
	sched scheduler.Scheduler) *Libp2pEndpoint {
	return &Libp2pEndpoint{
		ctx:     ctx,
		host:    host,
		sched:   sched,
		writers: sync.Pool{},
		readers: sync.Pool{},
	}
}

func getPeerID(id address.NodeID) (*peerid.PeerID, error) {
	if p, ok := id.(*peerid.PeerID); ok {
		return p, nil
	}
	return nil, endpoint.ErrInvalidPeer
}

func (e *Libp2pEndpoint) AsyncDialAndReport(ctx context.Context,
	id address.NodeID, reportFn DialReportFn) error {
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
			e.sched.EnqueueAction(ctx, ba.BasicAction(func(ctx context.Context) {
				reportFn(ctx, success)
			}))
		}
	}()
	return nil
}

func (e *Libp2pEndpoint) DialPeer(ctx context.Context, id address.NodeID) error {
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
	id address.NodeAddr, ttl time.Duration) error {
	_, span := util.StartSpan(ctx, "Libp2pEndpoint.MaybeAddToPeerstore",
		trace.WithAttributes(attribute.String("PeerID", id.NodeID().String())))
	defer span.End()

	ai, ok := id.(*addrinfo.AddrInfo)
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
	protoID address.ProtocolID, n address.NodeID, req message.MinKadMessage,
	resp message.MinKadMessage, timeout time.Duration,
	responseHandlerFn endpoint.ResponseHandlerFn) error {

	_, span := util.StartSpan(context.Background(),
		"Libp2pEndpoint.SendRequestHandleResponse", trace.WithAttributes(
			attribute.String("PeerID", n.String()),
		))
	defer span.End()

	protoResp, ok := resp.(message.ProtoKadResponseMessage)
	if !ok {
		span.RecordError(ErrRequireProtoKadResponse)
		return ErrRequireProtoKadResponse
	}

	protoReq, ok := req.(message.ProtoKadMessage)
	if !ok {
		span.RecordError(ErrRequireProtoKadMessage)
		return ErrRequireProtoKadMessage
	}

	p, ok := n.(*peerid.PeerID)
	if !ok {
		span.RecordError(ErrRequirePeerID)
		return ErrRequirePeerID
	}

	if responseHandlerFn == nil {
		span.RecordError(endpoint.ErrNilResponseHandler)
		return endpoint.ErrNilResponseHandler
	}

	go func() {
		ctx, span := util.StartSpan(context.Background(),
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
			e.sched.EnqueueAction(ctx, ba.BasicAction(func(ctx context.Context) {
				responseHandlerFn(ctx, nil, err)
			}))
			return
		}
		defer s.Close()

		err = WriteMsg(s, protoReq)
		if err != nil {
			span.RecordError(err, trace.WithAttributes(attribute.String("where", "write message")))
			e.sched.EnqueueAction(ctx, ba.BasicAction(func(ctx context.Context) {
				responseHandlerFn(ctx, nil, err)
			}))
			return
		}

		var timeoutEvent planner.PlannedAction
		// handle timeout

		if timeout != 0 {
			timeoutEvent = scheduler.ScheduleActionIn(ctx, e.sched, timeout,
				ba.BasicAction(func(ctx context.Context) {
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
			e.sched.EnqueueAction(ctx, ba.BasicAction(func(ctx context.Context) {
				responseHandlerFn(ctx, protoResp, err)
			}))
			return
		}

		span.AddEvent("response received")
		e.sched.EnqueueAction(ctx, ba.BasicAction(func(ctx context.Context) {
			responseHandlerFn(ctx, protoResp, err)
		}))
	}()
	return nil
}

func (e *Libp2pEndpoint) Connectedness(id address.NodeID) (network.Connectedness, error) {
	p, err := getPeerID(id)
	if err != nil {
		return network.NotConnected, err
	}
	return e.host.Network().Connectedness(p.ID), nil
}

func (e *Libp2pEndpoint) PeerInfo(id address.NodeID) (peer.AddrInfo, error) {
	p, err := getPeerID(id)
	if err != nil {
		return peer.AddrInfo{}, err
	}
	return e.host.Peerstore().PeerInfo(p.ID), nil
}

func (e *Libp2pEndpoint) KadKey() key.KadKey {
	return peerid.PeerID{ID: e.host.ID()}.Key()
}

func (e *Libp2pEndpoint) NetworkAddress(n address.NodeID) (address.NodeAddr, error) {
	ai, err := e.PeerInfo(n)
	if err != nil {
		return nil, err
	}
	return addrinfo.NewAddrInfo(ai), nil
}

func (e *Libp2pEndpoint) AddRequestHandler(protoID address.ProtocolID,
	req message.MinKadMessage, reqHandler endpoint.RequestHandlerFn) error {

	protoReq, ok := req.(message.ProtoKadMessage)
	if !ok {
		return ErrRequireProtoKadMessage
	}
	if reqHandler == nil {
		return endpoint.ErrNilRequestHandler
	}
	// when a new request comes in, we need to queue it
	streamHandler := func(s network.Stream) {
		e.sched.EnqueueAction(e.ctx, ba.BasicAction(func(ctx context.Context) {
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

				requester := addrinfo.NewAddrInfo(
					e.host.Peerstore().PeerInfo(s.Conn().RemotePeer()),
				)
				resp, err := reqHandler(ctx, requester, req)
				if err != nil {
					span.RecordError(err)
					return
				}

				protoResp, ok := resp.(message.ProtoKadMessage)
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
