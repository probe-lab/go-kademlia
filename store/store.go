package store

import (
	"context"

	lru "github.com/hashicorp/golang-lru/simplelru"
	ds "github.com/ipfs/go-datastore"
	"github.com/plprobelab/go-kademlia/key"
	"github.com/plprobelab/go-kademlia/network/address"
	"github.com/plprobelab/go-kademlia/util"
	"go.opentelemetry.io/otel/attribute"
)

type RecordStore interface {
	Put(ctx context.Context, messages ...Message) error
	Get(ctx context.Context, message Message) ([]Message, error)
}

type Record interface {
	Key() key.KadKey
	DatastoreKey() ds.Key
	DatastoreValue() []byte
}

type Store struct {
	self   address.NodeID
	cache  lru.LRUCache
	dstore ds.Datastore
}

var _ RecordStore = (*Store)(nil)

func New(ctx context.Context, self address.NodeID, dstore ds.Datastore, opts ...Option) (*Store, error) {
	cache, err := lru.NewLRU(lruCacheSize, nil)
	if err != nil {
		return nil, err
	}

	s := &Store{
		self:   self,
		cache:  cache,
		dstore: dstore,
	}

	return s, nil
}

func (s Store) AddRecords(ctx context.Context, records ...Record) error {
	ctx, span := util.StartSpan(ctx, "Store.AddRecords")
	span.SetAttributes(attribute.KeyValue{
		Key:   "count",
		Value: attribute.IntValue(len(records)),
	})
	defer span.End()

	for _, record := range records {
		_, isSelf := record.Provider().Key().Equal(s.self.Key())
		if isSelf {
			continue
		}

	}

	if s.self != pm.self { // don't add own addrs.
		pm.pstore.AddAddrs(provInfo.ID, provInfo.Addrs, ProviderAddrTTL)
	}
	prov := &addProv{
		ctx: ctx,
		key: k,
		val: provInfo.ID,
	}
	select {
	case pm.newprovs <- prov:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s Store) GetRecords(ctx context.Context, key []key.KadKey) ([]Record, error) {
}
