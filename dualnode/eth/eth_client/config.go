/*
 *  Copyright 2018 KardiaChain
 *  This file is part of the go-kardia library.
 *
 *  The go-kardia library is free software: you can redistribute it and/or modify
 *  it under the terms of the GNU Lesser General Public License as published by
 *  the Free Software Foundation, either version 3 of the License, or
 *  (at your option) any later version.
 *
 *  The go-kardia library is distributed in the hope that it will be useful,
 *  but WITHOUT ANY WARRANTY; without even the implied warranty of
 *  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 *  GNU Lesser General Public License for more details.
 *
 *  You should have received a copy of the GNU Lesser General Public License
 *  along with the go-kardia library. If not, see <http://www.gnu.org/licenses/>.
 */

package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"

	"github.com/kardiachain/go-kardiamain/lib/log"
)

type (
	Config struct {
		Name               string   `yaml:"Name"`
		ListenAddr         string   `yaml:"ListenAddr"`
		APIListenAddr      string   `yaml:"APIListenAddr"`
		MaxPeers           int      `yaml:"MaxPeers"`
		NetworkId          int      `yaml:"NetworkId"`
		LightNode          bool     `yaml:"LightNode"`
		LightPeers         int      `yaml:"LightPeers"`
		LightServ          int      `yaml:"LightServ"`
		StatName           string   `yaml:"StatName"`
		ReportStats        bool     `yaml:"ReportStats"`
		ContractAddress    []string `yaml:"ContractAddress"`
		ContractAbis       []string `yaml:"ContractAbis"`
		HTTPHost           string   `yaml:"HTTPHost"`
		HTTPPort           int      `yaml:"HTTPPort"`
		HTTPVirtualHosts   []string `yaml:"HTTPVirtualHosts"`
		HTTPCors           []string `yaml:"HTTPCors"`
		CacheSize          int      `yaml:"CacheSize"`
		DBHandle           int      `yaml:"DBHandle"`
		SubscribedEndpoint string   `yaml:"SubscribedEndpoint"`
		PublishedEndpoint  string   `yaml:"PublishedEndpoint"`
		SignedTxPrivateKey string   `yaml:"SignedTxPrivateKey"`
		LogLvl             int      `yaml:"LogLvl"`
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
