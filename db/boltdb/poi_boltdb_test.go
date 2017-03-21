package boltdb_test

import (
	"github.com/alpe/geostore/api"
	"github.com/alpe/geostore/db/boltdb"
	"io/ioutil"
	"testing"
)

func TestFindByRadius(t *testing.T) {
	db := testPOIDB(t)
	defer db.Close()

	lat := 39.578967 // Palma - Islas Baleares
	lng := 3.098145
	room := api.RoomIO{
		ID:       1,
		Currency: "EUR",
		Address:  "myAddress",
	}
	if err := db.Store(lat, lng, room); err != nil {
		t.Fatalf("unexpected error %s", err)
	}

	// when
	rooms, err := db.FindInRadius(lat, lng, 1)
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}

	// then
	if got, exp := len(rooms), 1; got != exp {
		t.Fatalf("expected %v but got %v", exp, got)
	}
	loadedRoom := rooms[0]
	if got, exp := loadedRoom.Room.ID, uint64(1); got != exp {
		t.Errorf("expected %v but got %v", exp, got)
	}
	if got, exp := loadedRoom.Room.Currency, "EUR"; got != exp {
		t.Errorf("expected %v but got %v", exp, got)
	}
	if got, exp := loadedRoom.Room.Address, "myAddress"; got != exp {
		t.Errorf("expected %v but got %v", exp, got)
	}

}

func BenchmarkFindByRadius(b *testing.B) {
	db := testPOIDB(b)
	defer db.Close()
	// with rooms
	lat := 39.578967 // Palma - Islas Baleares
	lng := 3.098145
	room := api.RoomIO{
		Currency: "EUR",
		Address:  "myAddress",
	}
	for i := 0; i < 1000; i++ {
		if err := db.Store(lat, lng+float64(i)/10000, room); err != nil {
			room.ID = uint64(1 + i)
			b.Fatalf("unexpected error %s", err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := db.FindInRadius(lat, lng, 15000)
		if err != nil {
			b.Fatalf("unexpected error %s", err)
		}
	}
}

func testPOIDB(b interface {
	Fatalf(string, ...interface{})
}) *boltdb.POIBoltDB {
	tmpfile, err := ioutil.TempFile("", "teststorage")
	if err != nil {
		b.Fatalf("unexpected error %s", err)
	}
	db, err := boltdb.New(tmpfile.Name())
	if err != nil {
		b.Fatalf("unexpected error %s", err)
	}
	return db
}
