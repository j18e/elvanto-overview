package repositories

import (
	"encoding/json"
	"errors"
	"fmt"

	bolt "go.etcd.io/bbolt"
)

var ErrNotFound = errors.New("not found")

var bucketName = []byte("tokens")

type Tokens struct {
	Access  string `json:"access_token"`
	Refresh string `json:"refresh_token"`
}

func NewRepository(file string) (*Repository, error) {
	db, err := bolt.Open(file, 0600, nil)
	if err != nil {
		return nil, err
	}
	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(bucketName)
		if err != nil {
			return fmt.Errorf("create bucket: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &Repository{db: db}, nil
}

type Repository struct {
	db *bolt.DB
}

func (r *Repository) Get(id string) (*Tokens, error) {
	var bs []byte
	r.db.View(func(tx *bolt.Tx) error {
		bs = tx.Bucket(bucketName).Get([]byte(id))
		return nil
	})
	if bs == nil {
		return nil, ErrNotFound
	}
	var tok Tokens
	if err := json.Unmarshal(bs, &tok); err != nil {
		return nil, err
	}
	return &tok, nil
}

func (r *Repository) Store(id string, tokens Tokens) error {
	bs, err := json.Marshal(&tokens)
	if err != nil {
		return err
	}
	return r.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketName).Put([]byte(id), bs)
	})
}
