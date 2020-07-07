package boltStore

import (
	"errors"
	"time"

	"github.com/aloknerurkar/gopcp/core"
	"github.com/boltdb/bolt"
	"github.com/gogo/protobuf/proto"
	"github.com/google/uuid"
)

const (
	BUCKET    = "main"
	SEPARATOR = "/"
)

func NewBoltStore(conf map[string]interface{}) (core.Store, error) {
	root, ok := conf["root"].(string)
	if !ok {
		return nil, errors.New("Root path missing in config")
	}

	dbName, ok := conf["dbName"].(string)
	if !ok {
		return nil, errors.New("DB name missing in config")
	}

	fullName := root + "/." + dbName + ".db"
	db, e := bolt.Open(fullName, 0600, nil)
	if e != nil {
		return nil, e
	}

	return &boltDB{
		dbP: db,
	}, nil
}

type boltDB struct {
	dbP *bolt.DB
}

func getKey(i core.Item) []byte {
	return []byte(i.GetTable() + SEPARATOR + i.GetId())
}

func (s *boltDB) Create(i core.Item) error {

	if v, ok := i.(core.IdSetter); ok {
		v.SetId(uuid.New().String())
	}

	if v, ok := i.(core.TimeTracker); ok {
		v.SetCreated(time.Now().Unix())
	}

	return s.Update(i)
}

func (s *boltDB) Update(i core.Item) error {
	if v, ok := i.(core.TimeTracker); ok {
		v.SetUpdated(time.Now().Unix())
	}

	if msg, ok := i.(proto.Message); ok {
		buf, err := proto.Marshal(msg)
		if err != nil {
			return err
		}

		return s.dbP.Update(func(tx *bolt.Tx) error {
			bkt, err := tx.CreateBucketIfNotExists([]byte(BUCKET))
			if err != nil {
				return err
			}
			return bkt.Put(getKey(i), buf)
		})
	}

	return errors.New("Invalid item type")
}

func (s *boltDB) Delete(i core.Item) error {
	return s.dbP.Update(func(tx *bolt.Tx) error {
		bkt := tx.Bucket([]byte(BUCKET))
		if bkt == nil {
			return errors.New("Failed opening bucket")
		}
		return bkt.Delete(getKey(i))
	})
}

func (s *boltDB) Read(i core.Item) error {
	return s.dbP.View(func(tx *bolt.Tx) error {
		bkt := tx.Bucket([]byte(BUCKET))
		if bkt == nil {
			return errors.New("Failed opening bucket")
		}

		val := bkt.Get(getKey(i))

		if msg, ok := i.(proto.Message); ok {
			return proto.Unmarshal(val, msg)
		}
		return errors.New("Invalid item type")
	})
}

func (s *boltDB) List(l core.Items, o core.ListOpt) (int, error) {
	idx := 0
	err := s.dbP.View(func(tx *bolt.Tx) error {
		bkt := tx.Bucket([]byte(BUCKET))
		if bkt == nil {
			// Bucket not created yet
			return nil
		}

		c := bkt.Cursor()

		skip := o.Page * o.Limit
		for k, v := c.Last(); k != nil; k, v = c.Prev() {
			skip--
			if skip > 0 {
				continue
			}
			if msg, ok := l[idx].(proto.Message); ok {
				err := proto.Unmarshal(v, msg)
				if err != nil {
					return err
				}
				idx++
			}
		}
		return nil
	})
	return idx, err
}

func (s *boltDB) ListAll(allocFn func(i int) core.Items) (core.Items, error) {
	valBufs := make([][]byte, 0)
	err := s.dbP.View(func(tx *bolt.Tx) error {
		bkt := tx.Bucket([]byte(BUCKET))
		if bkt == nil {
			return nil
		}

		c := bkt.Cursor()

		for k, v := c.Last(); k != nil; k, v = c.Prev() {
			valBufs = append(valBufs, v)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	l := allocFn(len(valBufs))
	for i, v := range l {
		if msg, ok := v.(proto.Message); ok {
			err := proto.Unmarshal(valBufs[i], msg)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, errors.New("Object allocated invalid")
		}
	}
	return l, nil
}
