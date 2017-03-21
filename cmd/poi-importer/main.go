package main

import (
	"encoding/csv"
	"flag"
	log "github.com/Sirupsen/logrus"
	"github.com/alpe/geostore/api"
	"github.com/alpe/geostore/db/boltdb"
	"io"
	"os"
	"strconv"
)

func main() {
	importPath := flag.String("import", "", "import file path")
	configPath := flag.String("dbpath", "", "config file path")
	flag.Parse()

	file, err := os.Open(*importPath)
	if err != nil {
		log.Fatalf("failed to open import file: %q: %s", *importPath, err)
	}
	defer file.Close()
	db, err := boltdb.New(*configPath)
	if err != nil {
		log.Fatalf("failed to open DB file: %q: %s", *configPath, err)
	}

	r := csv.NewReader(file)
	_, _ = r.Read() // skip header
	i := 0
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if i > 100 {
			break
		}
		lat, err := strconv.ParseFloat(record[7], 64)
		if err != nil {
			log.Fatalf("failed to convert %q to float64", record[7])
		}
		lng, err := strconv.ParseFloat(record[8], 64)
		if err != nil {
			log.Fatalf("failed to convert %q to float64", record[8])
		}
		id, err := strconv.ParseUint(record[0], 10, 64)
		if err != nil {
			log.Fatalf("failed to convert %q to uint64", record[8])
		}

		err = db.Store(lat, lng, api.RoomIO{
			ID:       id,
			Lat:      lat,
			Lng:      lng,
			Currency: record[9],
			Address:  record[6],
		})
		if err != nil {
			log.Fatalf("failed to persist row %d: %s", i, err)
		}
	}
}
