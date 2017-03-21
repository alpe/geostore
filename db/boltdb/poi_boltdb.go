package boltdb

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/alpe/geostore/api"
	"github.com/alpe/geostore/db/payload"
	"github.com/boltdb/bolt"
	"github.com/golang/geo/s2"
	"github.com/golang/protobuf/proto"
	lru "github.com/hashicorp/golang-lru"
	"math"
	"time"
)

var (
	defaultCoverer = s2.RegionCoverer{MinLevel: 1, MaxLevel: 30, MaxCells: 8}
)

const roomsBucket = "rooms"
const defaultCacheSize = 50000 // number of elements

type POIBoltDB struct {
	*bolt.DB
	cache *lru.Cache
}

func New(dbPath string) (*POIBoltDB, error) {
	db, err := bolt.Open(dbPath, 0600, &bolt.Options{ReadOnly: false})
	if err != nil {
		return nil, err
	}
	cache, err := lru.New(defaultCacheSize)
	if err != nil {
		return nil, err
	}

	pioDB := POIBoltDB{
		DB:    db,
		cache: cache,
	}
	if err := pioDB.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(roomsBucket))
		return err
	}); err != nil {
		return nil, fmt.Errorf("failed to init DB: %s", err)
	}
	return &pioDB, nil
}

func (db *POIBoltDB) Store(lat, lng float64, roomIO api.RoomIO) error {
	newRoom := payload.Room{
		Id:       roomIO.ID,
		Lat:      lat,
		Lng:      lng,
		Currency: roomIO.Currency,
		Address:  roomIO.Address,
	}
	cellID := s2.CellIDFromLatLng(s2.LatLngFromDegrees(lat, lng))
	return db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(roomsBucket))
		id := asKey(cellID)
		var rooms payload.Rooms
		if v := b.Get(id); v == nil {
			rooms.CellID = uint64(cellID)
			rooms.Rooms = make(map[uint64]*payload.Room)
		} else {
			if err := proto.Unmarshal(v, &rooms); err != nil {
				return fmt.Errorf("failed to unmarshal content of cellID %d: %s", cellID, err)
			}
		}
		rooms.Rooms[roomIO.ID] = &newRoom
		data, err := proto.Marshal(&rooms)
		if err != nil {
			return fmt.Errorf("failed to marshal content of cellID %d: %s", cellID, err)

		}
		db.cache.Add(cellID, rooms)
		return b.Put(id, data)
	})
}

func (db *POIBoltDB) FindAll() ([]api.RoomIO, error) {
	r := make([]api.RoomIO, 0)
	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(roomsBucket))
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var rooms payload.Rooms
			if err := proto.Unmarshal(v, &rooms); err != nil {
				return fmt.Errorf("failed to unmarshal content of key %s: %s", k, err)
			}
			for _, room := range rooms.Rooms {
				r = append(r, api.RoomIO{
					ID:       room.Id,
					Currency: room.Currency,
					Address:  room.Address,
					Lat:      room.Lat,
					Lng:      room.Lng,
				})
			}
		}
		return nil
	})
	return r, err
}

// FindInRadius
func (db *POIBoltDB) FindInRadius(lat, lng, radius float64) ([]api.RoomCellIO, error) {
	center := s2.PointFromLatLng(s2.LatLngFromDegrees(lat, lng))
	cap := s2.CapFromCenterArea(center, s2RadialAreaMeters(radius))
	return db.coveredRooms(defaultCoverer.Covering(cap))
}

// FindInRect
func (db *POIBoltDB) FindInRect(urLat, urLng, blLat, blLng float64) ([]api.RoomCellIO, error) {
	rect := s2.RectFromLatLng(s2.LatLngFromDegrees(blLat, blLng))
	rect = rect.AddPoint(s2.LatLngFromDegrees(urLat, urLng))
	rc := &s2.RegionCoverer{MaxLevel: 30, MaxCells: 4}
	return db.coveredRooms(rc.Covering(rect))

}
func (db *POIBoltDB) coveredRooms(covering s2.CellUnion) ([]api.RoomCellIO, error) {
	start := time.Now()
	if len(covering) == 0 {
		return nil, errors.New("impossible covering")
	}

	r := make([]api.RoomCellIO, 0)
	if err := db.View(func(tx *bolt.Tx) error {
		for _, cellID := range covering {
			rooms, err := db.poisInCellID(tx, cellID)
			if err != nil {
				return err
			}
			r = append(r, rooms...)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	log.Debugf("Find took %d ns\n", time.Now().Sub(start))
	return r, nil
}

func (db *POIBoltDB) poisInCellID(tx *bolt.Tx, cellID s2.CellID) ([]api.RoomCellIO, error) {
	r := make([]api.RoomCellIO, 0)
	b := tx.Bucket([]byte(roomsBucket))
	c := b.Cursor()
	// perform a range lookup in the DB from bmin key to bmax key, cur is our DB cursor
	for k, v := c.Seek(asKey(cellID.RangeMin())); k != nil && bytes.Compare(k, asKey(cellID.RangeMax())) <= 0; k, v = c.Next() {
		var rooms payload.Rooms
		if r, ok := db.cache.Get(fromKey(k)); ok {
			rooms = r.(payload.Rooms)
		} else {
			if err := proto.Unmarshal(v, &rooms); err != nil {
				return nil, fmt.Errorf("failed to unmarshal content of cellID %d: %s", cellID, err)
			}
			db.cache.Add(fromKey(k), rooms)
		}
		for _, room := range rooms.Rooms {
			r = append(r, api.RoomCellIO{
				CellID: cellID,
				Room: api.RoomIO{
					ID:       room.Id,
					Currency: room.Currency,
					Address:  room.Address,
					Lat:      room.Lat,
					Lng:      room.Lng,
				},
			})
		}

	}
	return r, nil
}

const earthCircumferenceMeter = 40075017

func s2RadialAreaMeters(radius float64) float64 {
	r := (radius / earthCircumferenceMeter) * math.Pi * 2
	return math.Pi * r * r
}

func asKey(c s2.CellID) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(c))
	return b
}

func fromKey(b []byte) s2.CellID {
	return s2.CellID(binary.BigEndian.Uint64(b))
}
