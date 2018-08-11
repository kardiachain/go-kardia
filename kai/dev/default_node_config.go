// Defines default configs used for initializing nodes in dev settings.

package dev

import (
	"crypto/ecdsa"

	"github.com/kardiachain/go-kardia/lib/crypto"
)

type privateKeyByte []byte

type DevNodeConfig struct {
	NodeKey *ecdsa.PrivateKey
}

type DevEnvironmentConfig struct {
	DevNodeSet []DevNodeConfig
}

var privKeySet []privateKeyByte

func init() {
	privKeyStringSet := []string{
		"8843ebcb1021b00ae9a644db6617f9c6d870e5fd53624cefe374c1d2d710fd06",
		"77cfc693f7861a6e1ea817c593c04fbc9b63d4d3146c5753c008cfc67cffca79",
		"98de1df1e242afb02bd5dc01fbcacddcc9a4d41df95a66f629139560ca6e4dbb",
		"32f5c0aef7f9172044a472478421c63fd8492640ff2d0eaab9562389db3a8efe",
		"68b53a92d846baafdc782cb9cad65d77020c8d747eca7b621370b52b18c91f9a",
		"049de018e08c3bcd59c1a21f0cf7de8f17fe51f8ce7d9c2120d17b1f0251b265",
		"9fdd56a3c2a536dc8f981d935f0f3f2ea04e125547fdfffa37e157ce86ff1007",
		"ae1a52546294bed6e734185775dbc84009de00bdf51b709471e2415c31ceeed7",
		"b34bd81838a4a335fb3403d0bf616eca1eb9a4b4716c7dda7c617503cfeaab67",
		"e049a09c992c882bc2deb780323a247c6ee0951f8b4c5c1dd0fc2fc22ce6493d",
	}

	privKeySet = make([]privateKeyByte, len(privKeyStringSet))
	for index, keyString := range privKeyStringSet {
		privKeySet[index] = privateKeyByte(keyString[:32])
	}
}

func CreateDevEnvironmentConfig() *DevEnvironmentConfig {
	var devEnv DevEnvironmentConfig
	devEnv.DevNodeSet = make([]DevNodeConfig, len(privKeySet))
	for index, privKeyByte := range privKeySet {
		nodeKey, _ := crypto.ToECDSA(privKeyByte)
		devEnv.DevNodeSet[index].NodeKey = nodeKey
	}

	return &devEnv
}

func (devEnv *DevEnvironmentConfig) GetDevNodeConfig(index int) *DevNodeConfig {
	return &devEnv.DevNodeSet[index]
}

func (devEnv *DevEnvironmentConfig) GetNodeSize() int {
	return len(devEnv.DevNodeSet)
}
