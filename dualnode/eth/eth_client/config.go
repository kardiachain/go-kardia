package main

import (
	"fmt"
	"github.com/ethereum/go-ethereum/log"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"path/filepath"
)

type (
	Config struct {
		Name               string      `yaml:"Name"`
		ListenAddr         string      `yaml:"ListenAddr"`
		APIListenAddr      string      `yaml:"APIListenAddr"`
		MaxPeers           int         `yaml:"MaxPeers"`
		NetworkId          int         `yaml:"NetworkId"`
		LightNode          bool        `yaml:"LightNode"`
		LightPeers         int         `yaml:"LightPeers"`
		LightServ          int         `yaml:"LightServ"`
		StatName           string      `yaml:"StatName"`
		ReportStats        bool        `yaml:"ReportStats"`
		ContractAddress    []string    `yaml:"ContractAddress"`
		ContractAbis       []string    `yaml:"ContractAbis"`
		HTTPHost           string      `yaml:"HTTPHost"`
		HTTPPort           int         `yaml:"HTTPPort"`
		HTTPVirtualHosts   []string    `yaml:"HTTPVirtualHosts"`
		HTTPCors           []string    `yaml:"HTTPCors"`
		CacheSize          int         `yaml:"CacheSize"`
		DBHandle           int         `yaml:"DBHandle"`
		SubscribedEndpoint string      `yaml:"SubscribedEndpoint"`
		PublishedEndpoint  string      `yaml:"PublishedEndpoint"`
		SignedTxPrivateKey string      `yaml:"SignedTxPrivateKey"`
		LogLvl             int         `yaml:"LogLvl"`
		Logger             log.Logger
	}
)

// Load attempts to load the config from given path and filename.
func Load(path string, name string) (*Config, error) {
	filename := fmt.Sprintf("%s.yml", name)
	configPath := filepath.Join(path, filename)
	log.Info(configPath)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, errors.Wrap(err, "Unable to load config")
	}
	configData, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, errors.Wrap(err, "Unable to read config")
	}

	config := Config{}

	err = yaml.Unmarshal([]byte(configData), &config)
	if err != nil {
		return nil, errors.Wrap(err, "Problem unmarshaling config json data")
	}

	return &config, nil
}

