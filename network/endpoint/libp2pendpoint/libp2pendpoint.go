package libp2pendpoint

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	ba "github.com/libp2p/go-libp2p-kad-dht/events/action/basicaction"
	"github.com/libp2p/go-libp2p-kad-dht/events/planner"
	"github.com/libp2p/go-libp2p-kad-dht/events/scheduler"
	"github.com/libp2p/go-libp2p-kad-dht/key"
	"github.com/libp2p/go-libp2p-kad-dht/network/address"
	"github.com/libp2p/go-libp2p-kad-dht/network/address/addrinfo"
	"github.com/libp2p/go-libp2p-kad-dht/network/address/peerid"
	"github.com/libp2p/go-libp2p-kad-dht/network/endpoint"
	"github.com/libp2p/go-libp2p-kad-dht/network/message"
	"github.com/libp2p/go-libp2p-kad-dht/util"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-msgio/pbio"
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

func NewMessageEndpoint(ctx context.Context, host host.Host,
	sched scheduler.Scheduler) *Libp2pEndpoint {
	return &Libp2pEndpoint{
		ctx:     ctx,
		host:    host,
		sched:   sched,
		writers: sync.Pool{},
		readers: sync.Pool{},
	}
}

func getPeerID(id address.NodeID) peerid.PeerID {
	if p, ok := id.(peerid.PeerID); ok {
		return p
	}
	panic("invalid peer id")
}

func (msgEndpoint *Libp2pEndpoint) AsyncDialAndReport(ctx context.Context,
	id address.NodeID, reportFn DialReportFn) {
	p := getPeerID(id)
	go func() {
		ctx, span := util.StartSpan(ctx, "Libp2pEndpoint.AsyncDialAndReport",
			trace.WithAttributes(attribute.String("PeerID", p.String())))
		defer span.End()

		success := true
		if err := msgEndpoint.DialPeer(ctx, p); err != nil {
			span.AddEvent("dial failed", trace.WithAttributes(
				attribute.String("Error", err.Error()),
			))
			success = false
		} else {
			span.AddEvent("dial successful")
		}

		// report dial result where it is needed
		reportFn(ctx, success)
	}()
}

func (msgEndpoint *Libp2pEndpoint) DialPeer(ctx context.Context, id address.NodeID) error {
	p := getPeerID(id)

	_, span := util.StartSpan(ctx, "Libp2pEndpoint.DialPeer", trace.WithAttributes(
		attribute.String("PeerID", p.String()),
	))
	defer span.End()

	if msgEndpoint.host.Network().Connectedness(p.ID) == network.Connected {
		span.AddEvent("Already connected")
		return nil
	}

	pi := peer.AddrInfo{ID: p.ID}
	if err := msgEndpoint.host.Connect(ctx, pi); err != nil {
		span.AddEvent("Connection failed", trace.WithAttributes(
			attribute.String("Error", err.Error()),
		))
		return err
	}
	span.AddEvent("Connection successful")
	return nil
}

func (msgEndpoint *Libp2pEndpoint) MaybeAddToPeerstore(ctx context.Context,
	id address.NodeID, ttl time.Duration) error {
	_, span := util.StartSpan(ctx, "Libp2pEndpoint.MaybeAddToPeerstore",
		trace.WithAttributes(attribute.String("PeerID", id.String())))
	defer span.End()

	ai, ok := id.(*addrinfo.AddrInfo)
	if !ok {
		return endpoint.ErrInvalidPeer
	}

	// Don't add addresses for self or our connected peers. We have better ones.
	if ai.ID == msgEndpoint.host.ID() ||
		msgEndpoint.host.Network().Connectedness(ai.ID) == network.Connected {
		return nil
	}
	msgEndpoint.host.Peerstore().AddAddrs(ai.ID, ai.Addrs, ttl)
	return nil
}

func (e *Libp2pEndpoint) SendRequestHandleResponse(ctx context.Context,
	protoID address.ProtocolID, n address.NodeID, req message.MinKadMessage,
	resp message.MinKadMessage, timeout time.Duration,
	responseHandlerFn endpoint.ResponseHandlerFn) {
	go func() {
		ctx, span := util.StartSpan(context.Background(), "Libp2pEndpoint.SendRequestHandleResponse", trace.WithAttributes(
			attribute.String("PeerID", n.String()),
		))

		defer span.End()

		var err error

		protoResp, ok := resp.(message.ProtoKadResponseMessage)
		if !ok {
			err = errors.New("Libp2pEndpoint requires ProtoKadResponseMessage")
			span.RecordError(err)
			responseHandlerFn(ctx, nil, err)
			return
		}

		protoReq, ok := req.(message.ProtoKadMessage)
		if !ok {
			err = errors.New("Libp2pEndpoint requires ProtoKadMessage")
			span.RecordError(err)
			responseHandlerFn(ctx, nil, err)
			return
		}

		p, ok := n.(*peerid.PeerID)
		if !ok {
			err = fmt.Errorf("Libp2pEndpoint requires peer.ID, %T", n)
			fmt.Printf("Libp2pEndpoint requires peer.ID, %T", n)

			span.RecordError(err)
			responseHandlerFn(ctx, nil, err)
			return
		}

		var s network.Stream
		s, err = e.host.NewStream(ctx, p.ID, protocol.ID(protoID))
		if err != nil {
			span.RecordError(err, trace.WithAttributes(attribute.String("where", "stream creation")))
			responseHandlerFn(ctx, nil, err)
			return
		}
		defer s.Close()

		err = WriteMsg(s, protoReq)
		if err != nil {
			span.RecordError(err, trace.WithAttributes(attribute.String("where", "write message")))
			responseHandlerFn(ctx, nil, err)
			return
		}

		var timeoutOccured bool
		var timeoutLock sync.Mutex
		var timeoutEvent planner.PlannedAction
		// handle timeout

		if timeout != 0 {
			timeoutEvent = scheduler.ScheduleActionIn(ctx, e.sched, timeout,
				ba.BasicAction(func(ctx context.Context) {
					timeoutLock.Lock()
					timeoutOccured = true
					timeoutLock.Unlock()

					responseHandlerFn(ctx, nil, endpoint.ErrTimeout)
				}))
		}

		err = ReadMsg(s, protoResp)
		if err != nil {
			span.RecordError(err, trace.WithAttributes(attribute.String("where", "read message")))
			responseHandlerFn(ctx, protoResp, err)
			return
		}
		if timeout != 0 {
			// remove timeout if not too late
			e.sched.RemovePlannedAction(ctx, timeoutEvent)

			timeoutLock.Lock()
			tooLate := timeoutOccured
			timeoutLock.Unlock()

			if tooLate {
				span.RecordError(fmt.Errorf("received response after timeout"))
				return
			}
		}

		span.AddEvent("response received")
		responseHandlerFn(ctx, protoResp, err)
	}()
}

func (msgEndpoint *Libp2pEndpoint) SendRequest(ctx context.Context,
	protoID address.ProtocolID, id address.NodeID, req message.MinKadMessage,
	resp message.MinKadMessage) error {

	protoReq, ok := req.(message.ProtoKadRequestMessage)
	if !ok {
		panic("Libp2pEndpoint requires ProtoKadRequestMessage")
	}
	protoResp, ok := resp.(message.ProtoKadResponseMessage)
	if !ok {
		panic("Libp2pEndpoint requires ProtoKadResponseMessage")
	}

	p := getPeerID(id)

	ctx, span := util.StartSpan(ctx, "Libp2pEndpoint.SendRequest", trace.WithAttributes(
		attribute.String("PeerID", p.String()),
	))
	defer span.End()

	s, err := msgEndpoint.host.NewStream(ctx, p.ID, protocol.ID(protoID))
	if err != nil {
		span.RecordError(err, trace.WithAttributes(attribute.String("where", "stream creation")))
		return err
	}
	defer s.Close()

	err = WriteMsg(s, protoReq)
	if err != nil {
		span.RecordError(err, trace.WithAttributes(attribute.String("where", "write message")))
		return err
	}

	err = ReadMsg(s, protoResp)
	if err != nil {
		span.RecordError(err, trace.WithAttributes(attribute.String("where", "read message")))
		return err
	}
	return nil
}

func (msgEndpoint *Libp2pEndpoint) Connectedness(id address.NodeID) network.Connectedness {
	p := getPeerID(id)
	return msgEndpoint.host.Network().Connectedness(p.ID)
}

func (msgEndpoint *Libp2pEndpoint) PeerInfo(id address.NodeID) peer.AddrInfo {
	p := getPeerID(id)
	return msgEndpoint.host.Peerstore().PeerInfo(p.ID)
}

func (e *Libp2pEndpoint) KadKey() key.KadKey {
	return peerid.PeerID{ID: e.host.ID()}.Key()
}

func (e *Libp2pEndpoint) NetworkAddress(n address.NodeID) (address.NodeID, error) {
	p, ok := n.(peerid.PeerID)
	if !ok {
		return nil, errors.New("invalid peer.ID")
	}
	return &addrinfo.AddrInfo{AddrInfo: e.host.Peerstore().PeerInfo(p.ID)}, nil
}

func (e *Libp2pEndpoint) AddRequestHandler(protoID address.ProtocolID,
	reqHandler endpoint.RequestHandlerFn) {

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
				var req message.ProtoKadMessage
				err := r.ReadMsg(req)
				if err != nil {
					if err == io.EOF {
						// stream EOF, all done
						return
					}
					span.RecordError(err)
					return
				}

				requester := peerid.NewPeerID(s.Conn().RemotePeer())
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
}

func (e *Libp2pEndpoint) RemoveRequestHandler(protoID address.ProtocolID) {
	e.host.RemoveStreamHandler(protocol.ID(protoID))
}
