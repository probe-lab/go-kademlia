package store

import lru "github.com/hashicorp/golang-lru/simplelru"

type Option func(*Store) error

func Cache(c lru.LRUCache) Option {
	return func(s *Store) error {
		s.cache = c
		return nil
	}
}

func CacheSize(size int) Option {
	return func(s *Store) error {
		s.cache.Resize(size)
		return nil
	}
}
