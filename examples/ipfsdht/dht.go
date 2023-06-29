package ipfsdht

import (
	"context"
	"sync"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/plprobelab/go-kademlia/events/scheduler/simplescheduler"
	"github.com/plprobelab/go-kademlia/network/address/addrinfo"
	"github.com/plprobelab/go-kademlia/network/address/peerid"
	"github.com/plprobelab/go-kademlia/network/endpoint/libp2pendpoint"
	sq "github.com/plprobelab/go-kademlia/query/simplequery"
	"github.com/plprobelab/go-kademlia/routingtable/simplert"
)

type IpfsDHT struct {
	pid   *peerid.PeerID
	rt    *simplert.SimpleRT
	ep    *libp2pendpoint.Libp2pEndpoint
	sched *simplescheduler.SimpleScheduler

	queryOpts []sq.Option

	lk                     sync.Mutex
	ongoingFindPeerQueries map[*peerid.PeerID]*sq.SimpleQuery
}

func New(ctx context.Context, h host.Host) *IpfsDHT {
	clk := clock.New()
	sched := simplescheduler.NewSimpleScheduler(clk)

	pid := peerid.NewPeerID(h.ID())
	rt := simplert.NewSimpleRT(pid.Key(), 20)
	ep := libp2pendpoint.NewLibp2pEndpoint(ctx, h, sched)

	queryOpts := []sq.Option{
		sq.WithProtocolID("/ipfs/kad/1.0.0"),
		sq.WithConcurrency(10),
		sq.WithNumberUsefulCloserPeers(20),
		sq.WithRequestTimeout(time.Second),
		sq.WithRoutingTable(rt),
		sq.WithEndpoint(ep),
		sq.WithScheduler(sched),
	}

	dht := &IpfsDHT{
		pid:   pid,
		rt:    rt,
		ep:    ep,
		sched: sched,

		queryOpts: queryOpts,

		lk:                     sync.Mutex{},
		ongoingFindPeerQueries: make(map[*peerid.PeerID]*sq.SimpleQuery),
	}

	dht.Run(ctx)

	return dht
}

func (dht *IpfsDHT) Connect(ctx context.Context, ai peer.AddrInfo) {
	addr := addrinfo.NewAddrInfo(ai)
	dht.ep.MaybeAddToPeerstore(ctx, addr, 30*time.Minute)
	dht.rt.AddPeer(ctx, addr.PeerID())
}

func (dht *IpfsDHT) Run(ctx context.Context) {
	go func() {
		for {
			for dht.sched.RunOne(ctx) {
			}
			time.Sleep(time.Millisecond * 100)
		}
	}()
}

func (dht *IpfsDHT) OngoingQueries() int {
	dht.lk.Lock()
	defer dht.lk.Unlock()

	return len(dht.ongoingFindPeerQueries)
}
