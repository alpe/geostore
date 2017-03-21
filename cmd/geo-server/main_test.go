package main

import (
	"fmt"
	"github.com/alpe/geostore/api"
	"github.com/golang/geo/s2"
	"github.com/pquerna/ffjson/ffjson"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestQueryByRadius(t *testing.T) {
	myRoom := api.RoomIO{
		ID:       1,
		Currency: "EUR",
		Lat:      51.9244,
		Lng:      4.4777,
		Address:  "Rotterdam",
	}

	s := &server{
		poiDB: &mockDB{
			findInRadius: func(lat, lng, radius float64) ([]api.RoomCellIO, error) {
				return []api.RoomCellIO{
					{
						CellID: s2.CellID(1.),
						Room:   myRoom,
					},
				}, nil
			},
		},
	}

	roomsURL := fmt.Sprintf("/?lat=%f&lng=%f&radius=%d&limit=1", 51.9244, 4.4777, 1000)
	r, err := http.NewRequest("GET", roomsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	// when
	s.roomsInRadius(w, r)

	// then
	if got, exp := w.Code, 200; exp != got {
		t.Errorf("expected %d but got %d: %s", exp, got, w.Body.String())
	}
	data := api.DataFrame{Data: []api.RoomCellIO{}}
	if err := ffjson.Unmarshal(w.Body.Bytes(), &data); err != nil {
		t.Fatal(err)
	}
	if got, exp := len(data.Data), 1; exp != got {
		t.Errorf("expected %v but got %v", exp, got)
	}
	cell := data.Data[0]
	if got, exp := cell.CellID, s2.CellID(1.); exp != got {
		t.Errorf("expected %v but got %v", exp, got)
	}
	if !reflect.DeepEqual(myRoom, cell.Room) {
		t.Errorf("expected %#v but got %@v", myRoom, cell.Room)
	}
}

type mockDB struct {
	findInRadius func(lat, lng, radius float64) ([]api.RoomCellIO, error)
}

func (m *mockDB) FindInRadius(lat, lng, radius float64) ([]api.RoomCellIO, error) {
	return m.findInRadius(lat, lng, radius)
}

func (m *mockDB) FindInRect(urLat, urLng, blLat, blLng float64) ([]api.RoomCellIO, error) {
	panic("not implementd")

}
