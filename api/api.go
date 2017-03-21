//go:generate ffjson $GOFILE
package api

import (
	"github.com/golang/geo/s2"
	"github.com/kpawlik/geojson"
)

type DataFrame struct {
	Data []RoomCellIO `json:"data"`
}

type RoomIO struct {
	ID       uint64
	Currency string
	Address  string
	Lat, Lng float64
}

type GeoJsonFeatures geojson.FeatureCollection

type RoomCellIO struct {
	Room   RoomIO
	CellID s2.CellID
}

func (r RoomCellIO) ToGeoJSON() *GeoJsonFeatures {
	var geo GeoJsonFeatures
	cell := s2.CellFromCellID(r.CellID)
	cs := make([]geojson.Coordinate, 5)
	for i := 0; i < 4; i++ {
		p := cell.Vertex(i)
		ll := s2.LatLngFromPoint(p)
		cs[i] = geojson.Coordinate{
			geojson.CoordType(ll.Lng.Degrees()),
			geojson.CoordType(ll.Lat.Degrees()),
		}
	}
	cs[4] = cs[0] // first and last point equal for google maps
	poly := &geojson.Polygon{
		Type:        "Polygon",
		Coordinates: []geojson.Coordinates{cs},
	}

	properties := make(map[string]interface{})

	//for k, v := range f.Data {
	//	properties[k] = v
	//}

	geo.Features = []*geojson.Feature{
		{
			Type:       "Feature",
			Geometry:   poly,
			Properties: properties,
		},
	}
	geo.Type = "FeatureCollection"
	return &geo
}
