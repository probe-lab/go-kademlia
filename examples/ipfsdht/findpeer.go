package ipfsdht

import (
	"context"
	"fmt"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/plprobelab/go-kademlia/network/address"
	"github.com/plprobelab/go-kademlia/network/address/addrinfo"
	"github.com/plprobelab/go-kademlia/network/address/peerid"
	"github.com/plprobelab/go-kademlia/network/message"
	"github.com/plprobelab/go-kademlia/network/message/ipfsv1"
	"github.com/plprobelab/go-kademlia/query/simplequery"
)

func (dht *IpfsDHT) FindPeer(ctx context.Context, p peer.ID) (peer.AddrInfo, error) {

	pid := peerid.NewPeerID(p)
	req := ipfsv1.FindPeerRequest(pid)

	resChan := make(chan *peer.AddrInfo, 1)

	var stopQuery bool

	dialResponseHandler := func(ctx context.Context, success bool) {
		if success {
			na, err := dht.ep.NetworkAddress(pid)
			if err != nil {
				return
			}
			ai, ok := na.(*addrinfo.AddrInfo)
			if !ok {
				panic("invalid network address type")
			}
			stopQuery = true
			resChan <- &ai.AddrInfo
		}
	}

	responseHandler := func(ctx context.Context, id address.NodeID,
		resp message.MinKadResponseMessage) (bool, []address.NodeID) {
		// parse response to ipfs dht message
		msg, ok := resp.(*ipfsv1.Message)
		if !ok {
			fmt.Println("invalid response!")
			return false, nil
		}
		peers := make([]address.NodeID, 0, len(msg.CloserPeers))
		for _, p := range msg.CloserPeers {
			addrInfo, err := ipfsv1.PBPeerToPeerInfo(p)
			if err != nil {
				fmt.Println("invalid peer info format")
				continue
			}
			peers = append(peers, addrInfo.PeerID())
			if addrInfo.PeerID().ID == pid.ID {
				resChan <- &addrInfo.AddrInfo

				dht.ep.AsyncDialAndReport(ctx, pid, dialResponseHandler)
				return true, nil
			}
		}
		return stopQuery, peers
	}

	dht.lk.Lock()
	dht.ongoingFindPeerQueries[pid] = simplequery.NewSimpleQuery(ctx, req,
		append(dht.queryOpts, simplequery.WithHandleResultsFunc(responseHandler))...)
	dht.lk.Unlock()

	// wait for query to finish
	select {
	case <-ctx.Done():
		dht.lk.Lock()
		delete(dht.ongoingFindPeerQueries, pid)
		dht.lk.Unlock()

		return peer.AddrInfo{}, ctx.Err()
	case res := <-resChan:
		dht.lk.Lock()
		delete(dht.ongoingFindPeerQueries, pid)
		dht.lk.Unlock()

		if res == nil {
			return peer.AddrInfo{}, fmt.Errorf("peer %s not found", p)
		}
		return *res, nil
	}

}
