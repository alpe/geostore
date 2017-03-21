package config

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
)

type BoltDB struct {
	FilePath string `yaml:"filePath"`
}

type Server struct {
	Port string `yaml:"port"`
}

type Settings struct {
	GoogleMapsApiKey string `yaml:"googleMapsApiKey"`
	HttpServer       Server `yaml:"httpServer"`
	DB               BoltDB `yaml:"boltDB"`
}

func ReadFile(configPath string) (Settings, error) {
	var c Settings
	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		return c, fmt.Errorf("failed to read config file %q: %s", configPath, err)
	}
	err = yaml.Unmarshal(data, &c)
	return c, err
}
