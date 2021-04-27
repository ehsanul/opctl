package core

import (
	"context"
	"encoding/json"
	"os"
	"path"
	"strings"
	"sync"

	"github.com/dgraph-io/badger/v2"
	"github.com/opctl/opctl/sdks/go/model"
)

// stateStore allows efficiently querying the current state of opctl.
//
// State is materialized by applying events in the order in which they are received.
//
// efficient startup:
// A lastAppliedEventTimestamp is maintained and used at startup to pickup applying events
// from where we left off.
type stateStore interface {
	// TryGetCreds returns creds for a ref if any exist
	TryGetAuth(resource string) *model.Auth

	AddAuth(req model.AuthAdded) error
}

func newStateStore(
	ctx context.Context,
	dataDirPath string,
) (stateStore, error) {
	dbPath := path.Join(dataDirPath, "db")
	if err := os.MkdirAll(dbPath, 0700); nil != err {
		return nil, err
	}

	// a readonly db connection can't be established until
	// a manifest file is generated. This will create one
	db, _ := badger.Open(badger.DefaultOptions(dbPath).WithLogger(nil))
	// ignore the error here - it's probably because another process is running and
	// has already done this. If there's a legit issue it will be caught when
	// the readonly connection is made
	if db != nil {
		db.Close()
	}

	readonlyDb, err := badger.Open(
		badger.
			DefaultOptions(dbPath).
			WithReadOnly(true).
			WithLogger(nil),
	)
	if nil != err {
		return nil, err
	}

	writeToDB := func(cb func(*badger.DB) error) error {
		db, err := badger.Open(
			badger.
				DefaultOptions(dbPath).
				WithReadOnly(true).
				WithLogger(nil),
		)
		if err != nil {
			return err
		}
		defer db.Close()
		return cb(db)
	}

	return &_stateStore{
		authsByResourcesKeyPrefix:    "authsByResources_",
		callsByID:                    make(map[string]*model.Call),
		readonlyDB:                   readonlyDb,
		writeToDB:                    writeToDB,
		lastAppliedEventTimestampKey: "lastAppliedEventTimestamp",
	}, nil
}

type _stateStore struct {
	lastAppliedEventTimestampKey string
	authsByResourcesKeyPrefix    string
	callsByID                    map[string]*model.Call
	readonlyDB                   *badger.DB
	writeToDB                    func(func(*badger.DB) error) error
	// synchronize access via mutex
	mux sync.RWMutex
}

func (ss *_stateStore) AddAuth(authAdded model.AuthAdded) error {
	return ss.writeToDB(func(db *badger.DB) error {
		return db.Update(func(txn *badger.Txn) error {
			auth := authAdded.Auth
			encodedAuth, err := json.Marshal(auth)
			if err != nil {
				return err
			}

			return txn.Set(
				[]byte(ss.authsByResourcesKeyPrefix+strings.ToLower(auth.Resources)),
				encodedAuth,
			)
		})
	})
}

func (ss *_stateStore) TryGetAuth(
	ref string,
) *model.Auth {
	ref = strings.ToLower(ref)
	var auth *model.Auth
	ss.readonlyDB.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		it := txn.NewIterator(opts)
		defer it.Close()
		prefixBytes := []byte(ss.authsByResourcesKeyPrefix)
		for it.Seek(prefixBytes); it.ValidForPrefix(prefixBytes); it.Next() {
			item := it.Item()
			key := string(item.Key())
			prefix := strings.TrimPrefix(key, ss.authsByResourcesKeyPrefix)

			if strings.HasPrefix(ref, prefix) {
				item.Value(func(value []byte) error {
					auth = &model.Auth{}
					return json.Unmarshal(value, auth)
				})
			}
		}
		return nil
	})

	return auth
}
