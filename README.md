# Geo location data store

## Development
### Server
Build binary or run:
`go run ./cmd/geo-server/main.go -cfgpath="$(pwd)/config.yaml"`

### Configuration
Ceate a yaml file and pass it via `-cfgpath` parameter. Example
```yaml
googleMapsApiKey: "your-key"
httpServer:
  port: 8080
boltDB:
  filePath: "./poiimport.db"
```

### Dependencies
Install via
`glide install`
Add new 
`glide get github.com/foo/bar`

### Code generation
`go generate $(glide novendor)`


### Run Test + Benchmarks
`go test -bench=. $(glide novendor)`


## Query POIs
GET /query?lat=19.542915&lng=-155.665857&r=

## Resources: 
* Google maps API
https://developers.google.com/maps/documentation/javascript/3.exp/reference
* BoltDB
https://github.com/boltdb/bolt
* S2
http://blog.christianperone.com/2015/08/googles-s2-geometry-on-the-sphere-cells-and-hilbert-curve/
* A fast geo database with Google S2 take #2
http://blog.nobugware.com/post/2016/geo_db_s2_geohash_database/
## Inspired by
* https://github.com/akhenakh/regionagogo