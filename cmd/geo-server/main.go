package main

import (
	"flag"
	log "github.com/Sirupsen/logrus"
	"github.com/alpe/geostore/config"
	"html/template"
	"net/http"

	"fmt"
	"github.com/alpe/geostore/api"
	"github.com/alpe/geostore/db/boltdb"
	"github.com/golang/geo/s2"
	"github.com/kpawlik/geojson"
	"github.com/pquerna/ffjson/ffjson"
	"math"
	_ "net/http/pprof"
	"os"
	"sort"
	"strconv"
	"time"
)

type server struct {
	settings config.Settings
	poiDB    interface {
		FindInRadius(lat, lng, radius float64) ([]api.RoomCellIO, error)
		FindInRect(urLat, urLng, blLat, blLng float64) ([]api.RoomCellIO, error)
	}
}

func (s *server) index(w http.ResponseWriter, r *http.Request) {
	indexTemplate := template.Must(template.ParseFiles("cmd/geo-server/index.html"))
	model := map[string]string{
		"googleMapsApiKey": s.settings.GoogleMapsApiKey,
		"centerLat":        "51.9244", // Rotterdam
		"centerLng":        "4.4777",
		//"centerLat":        "39.578967", // palma
		//"centerLng":        "3.098145",
	}

	if err := indexTemplate.Execute(w, model); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *server) roomsInRadius(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	q, err := parseRadiusQuery(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	rooms, err := s.poiDB.FindInRadius(q.lat, q.lng, q.radius)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	centerSorter := &distToCenterSorter{
		center: s2.PointFromLatLng(s2.LatLngFromDegrees(q.lat, q.lng)),
		rooms:  rooms,
	}
	sort.Sort(centerSorter)
	defer log.Debugf("/rooms: Rooms returned: %d in %d ns", len(rooms), time.Now().Sub(start))

	limit := q.limit
	rooms = centerSorter.rooms
	if len(rooms) < limit {
		limit = len(rooms)
	}

	w.Header().Set("Content-Type", "application/json")
	enc := ffjson.NewEncoder(w)
	if err := enc.Encode(api.DataFrame{Data: rooms[0:limit]}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

type distToCenterSorter struct {
	center s2.Point
	rooms  []api.RoomCellIO
}

func (x distToCenterSorter) Len() int {
	return len(x.rooms)
}
func (x distToCenterSorter) Less(i, j int) bool {
	iToCenter := x.center.Distance(s2.PointFromLatLng(s2.LatLngFromDegrees(x.rooms[i].Room.Lat, x.rooms[i].Room.Lng))).Abs().Radians()
	jToCenter := x.center.Distance(s2.PointFromLatLng(s2.LatLngFromDegrees(x.rooms[j].Room.Lat, x.rooms[j].Room.Lng))).Abs().Radians()
	return iToCenter < jToCenter
}

func (x distToCenterSorter) Swap(i, j int) {
	x.rooms[i], x.rooms[j] = x.rooms[j], x.rooms[i]
}

func (s *server) roomsInRect(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	q, err := parseRectQuery(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	rooms, err := s.poiDB.FindInRect(q.upperRightLat, q.upperRightLng, q.bottomLeftLat, q.bottomLeftLng)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer log.Debugf("/rooms: Rooms returned: %d in %d ns", len(rooms), time.Now().Sub(start))
	w.Header().Set("Content-Type", "application/json")
	enc := ffjson.NewEncoder(w)
	if err := enc.Encode(api.DataFrame{Data: rooms}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

var flexCoverer = s2.RegionCoverer{MinLevel: 1, MaxLevel: 15, MaxCells: 8}

func (s *server) cells(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	q, err := parseRadiusQuery(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	center := s2.PointFromLatLng(s2.LatLngFromDegrees(q.lat, q.lng))
	cap := s2.CapFromCenterArea(center, s2RadialAreaMeters(q.radius))

	var geo api.GeoJsonFeatures
	geo.Type = "FeatureCollection"

	cells := flexCoverer.CellUnion(cap)
	geo.Features = make([]*geojson.Feature, len(cells))
	for i, c := range cells {
		cell := s2.CellFromCellID(c)
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
		geo.Features[i] = &geojson.Feature{
			Type:       "Feature",
			Geometry:   poly,
			Properties: properties,
		}
	}

	defer log.Debugf("/cells: Cells returned: %d in %d ns", len(cells), time.Now().Sub(start))

	w.Header().Set("Content-Type", "application/json")
	enc := ffjson.NewEncoder(w)
	if err := enc.Encode(geo); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// TODO: deduplicate
const earthCircumferenceMeter = 40075017

// TODO: deduplicate
func s2RadialAreaMeters(radius float64) float64 {
	r := (radius / earthCircumferenceMeter) * math.Pi * 2
	return math.Pi * r * r
}

func main() {
	configPath := flag.String("cfgpath", "", "config file path")
	flag.Parse()

	log.SetFormatter(&log.JSONFormatter{})
	log.SetOutput(os.Stdout)
	log.SetLevel(log.DebugLevel)

	c, err := config.ReadFile(*configPath)
	if err != nil {
		log.Fatalf("failed to read config file %q: %s", configPath, err)
	}
	log.Debugf("Configuriation uses: %#v", c)
	db, err := boltdb.New(c.DB.FilePath)
	if err != nil {
		log.Fatalf("failed to setup DB: %s", err)
	}

	s := &server{
		settings: c,
		poiDB:    db,
	}
	http.HandleFunc("/cells", s.cells)
	http.HandleFunc("/rooms", s.roomsInRadius)
	http.HandleFunc("/", s.index)

	log.Debugf("Starting server on port: %q", c.HttpServer.Port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", c.HttpServer.Port), nil))
}

type poiRadiusQuery struct {
	lat, lng, radius float64
	limit            int
}

func parseRadiusQuery(r *http.Request) (q poiRadiusQuery, err error) {
	query := r.URL.Query()
	q.lat, err = strconv.ParseFloat(query.Get("lat"), 64)
	if err != nil {
		return q, err
	}
	q.lng, err = strconv.ParseFloat(query.Get("lng"), 64)
	if err != nil {
		return q, err
	}
	q.radius, err = strconv.ParseFloat(query.Get("radius"), 64)
	if err != nil {
		return q, err
	}
	q.limit, err = strconv.Atoi(query.Get("limit"))
	return q, err
}

type poiRectQuery struct {
	upperRightLat, upperRightLng float64
	bottomLeftLat, bottomLeftLng float64
}

func parseRectQuery(r *http.Request) (q poiRectQuery, err error) {
	query := r.URL.Query()
	q.upperRightLat, err = strconv.ParseFloat(query.Get("urLat"), 64)
	if err != nil {
		return q, err
	}
	q.upperRightLng, err = strconv.ParseFloat(query.Get("urLng"), 64)
	if err != nil {
		return q, err
	}
	q.bottomLeftLat, err = strconv.ParseFloat(query.Get("blLat"), 64)
	if err != nil {
		return q, err
	}
	q.bottomLeftLng, err = strconv.ParseFloat(query.Get("blLng"), 64)
	if err != nil {
		return q, err
	}
	return q, err
}
