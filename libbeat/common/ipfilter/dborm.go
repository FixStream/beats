package ipfilter

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/boltdb/bolt"
	"github.com/elastic/beats/libbeat/logp"
)

// DBorm type to have orm functionalites
type DBorm struct {
	db *bolt.DB
}

// Get value of given bucket and key
func (d *DBorm) Get(bucket, key string) (data []byte, err error) {
	d.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		r := b.Get([]byte(key))
		if r != nil {
			data = make([]byte, len(r))
			copy(data, r)
		}
		return nil
	})
	return
}

// GetAll key values
func (d *DBorm) GetAll(bucket string) (jsons []map[string]string, err error) {
	var m = make(map[string]string)
	d.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			m = map[string]string{string(k): string(v)}
			jsons = append(jsons, m)
		}
		return nil
	})
	return
}

// GetJSON json value of given bucket and key
func (d *DBorm) GetJSON(bucket, key string, v *IpRule) error {
	if data, ok := d.Get(bucket, key); ok == nil {
		//	logp.Info("getJsonorgid3 %s", data)
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		//	logp.Info("getJsonorgid1 %v", v)
		return nil
	}
	//	logp.Info("getJsonorgid2 %v", v)
	return nil
}

//Put key and value to db
func (d *DBorm) Put(bucket, key string, val []byte) (err error) {
	return d.db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(bucket))
		if err != nil {
			return err
		}
		return b.Put([]byte(key), val)
	})
}

//Delete key and value to db
func (d *DBorm) Delete(bucket, key string) (err error) {
	return d.db.Update(func(tx *bolt.Tx) error {
		if b := tx.Bucket([]byte(bucket)); b != nil {
			b.Delete([]byte(key))
		} else {
			return errors.New("Bucket not available to delete key")
		}
		return nil
	})
}

// NewDBORM creates new boltdb orm
func NewDBORM(path string) (orm *DBorm, err error) {
	roleDB, err := bolt.Open(path, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		logp.Err("fails to read the db: %v", err)
		return nil, err
	}
	err = roleDB.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(BucketName))
		if err != nil {
			logp.Err("create bucket: %v", err)
			return err
		}
		return nil
	})
	logp.Info("boltdb started")
	orm = &DBorm{roleDB}
	// defer roleDB.Close()
	return orm, nil
}
