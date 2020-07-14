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

package account

import (
	"bytes"
	"crypto/aes"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/kardiachain/go-kardiamain/lib/crypto"
	"golang.org/x/crypto/scrypt"
)

type KeyStoreJson struct {
	Address    string `json:"address"`
	Cipher     string `json:"cipher"`
	KDF        string `json:"kdf"`
	CipherText string `json:"cipherText"`
	IV         string `json:"iv"`
	Salt       string `json:"salt"`
	MAC        string `json:"mac"`
	TimeStamp  int64  `json:"timestamp"`
	Version    int8   `json:"version"`
}

type DecodedKeyStoreJson struct {
	CipherText []byte
	IV         []byte
	Salt       []byte
	MAC        []byte
}

var ErrDecrypt = errors.New("could not decrypt key with given passphrase")

func (keyStore *KeyStoreJson) decode() (*DecodedKeyStoreJson, error) {
	cipherText, err := hex.DecodeString(keyStore.CipherText)
	if err != nil {
		return nil, err
	}

	iv, err := hex.DecodeString(keyStore.IV)
	if err != nil {
		return nil, err
	}

	salt, err := hex.DecodeString(keyStore.Salt)
	if err != nil {
		return nil, err
	}

	mac, err := hex.DecodeString(keyStore.MAC)
	if err != nil {
		return nil, err
	}

	return &DecodedKeyStoreJson{CipherText: cipherText, IV: iv, Salt: salt, MAC: mac}, nil
}

func (keyStore *KeyStoreJson) GetPrivateKey(auth string) (*ecdsa.PrivateKey, error) {
	decoded, err := keyStore.decode()
	if err != nil {
		return nil, err
	}

	derivedKey, err := scrypt.Key([]byte(auth), decoded.Salt, scryptN, scryptR, scryptP, scryptDKLen)
	if err != nil {
		return nil, err
	}

	mac := crypto.Keccak256(derivedKey[16:32], decoded.CipherText, decoded.IV)
	if !bytes.Equal(mac, decoded.MAC) {
		return nil, ErrDecrypt
	}

	privateKey, err := aesCTRXOR(derivedKey[:16], decoded.CipherText, decoded.IV)
	if err != nil {
		return nil, err
	}

	return crypto.ToECDSA(privateKey)

}

/*
	Add marshaled keystoreJson into filename
*/
func (keystore *KeyStoreJson) StoreKey(filename string) error {
	content, err := json.Marshal(keystore)
	if err != nil {
		return err
	}
	return keystore.writeKeyFile(filename, content)
}

/*
	Get private key from derivedKey, cipherText, iv
*/
func GetPrivateKey(derivedKey, cipherText, iv []byte) (*ecdsa.PrivateKey, error) {

	key := derivedKey[:16]
	privateKey, err := aesCTRXOR(key, cipherText[aes.BlockSize:], iv)

	if err != nil {
		return nil, err
	}

	return crypto.ToECDSAUnsafe(privateKey), nil
}
