package account

import (
	"crypto/aes"
	"go-ethereum/crypto/randentropy"
	"crypto/ecdsa"
	"go-kardia/lib/crypto"
	crand "crypto/rand"
	"encoding/hex"
	"go-kardia/lib/crypto/sha3"
	"golang.org/x/crypto/scrypt"
	"go-kardia/lib/math"
	"crypto/cipher"
	"time"
	"os"
	"path/filepath"
	"io/ioutil"
	"encoding/json"
	"fmt"
)

const (
	kdfHeader = "scrypt"
	scryptN = 1 << 18
	scryptR = 1 << 3
	scryptP = 1
	scryptDKLen = 1 << 5
	AddressLength = 20
)


type KeyStore struct {
	Path string
	Address Address
	PrivateKey ecdsa.PrivateKey
}


type Address [AddressLength]byte

// Hex returns an EIP55-compliant hex string representation of the address.
func (a Address) Hex() string {
	unchecksummed := hex.EncodeToString(a[:])
	sha := sha3.NewKeccak256()
	sha.Write([]byte(unchecksummed))
	hash := sha.Sum(nil)

	result := []byte(unchecksummed)
	for i := 0; i < len(result); i++ {
		hashByte := hash[i/2]
		if i%2 == 0 {
			hashByte = hashByte >> 4
		} else {
			hashByte &= 0xf
		}
		if result[i] > '9' && hashByte > 7 {
			result[i] -= 32
		}
	}
	return "0x" + string(result)
}


/*
	Create new keystore based on path, password
 */
func (keyStore *KeyStore)createKeyStore(auth string) (bool, error) {
	// convert auth (password) to byte array
	authArray := []byte(auth)

	// get random iv
	iv := randentropy.GetEntropyCSPRNG(aes.BlockSize)

	// get random private key
	privateKey, err := ecdsa.GenerateKey(crypto.S256(), crand.Reader)
	if err != nil {
		return false, err
	}
	// get address from private key
	keyStore.PrivateKey = *privateKey
	keyStore.Address = Address(crypto.PubkeyToAddress(privateKey.PublicKey))

	// get random salt
	salt := randentropy.GetEntropyCSPRNG(scryptDKLen)

	// derived key - cipher text
	derivedKey, err := scrypt.Key(authArray, salt, scryptN, scryptR, scryptP, scryptDKLen)
	if err != nil {
		return false, err
	}

	// generate encrypted key, cipher text and mac
	encryptKey := derivedKey[:16]
	keyBytes := math.PaddedBigBytes(privateKey.D, 32)
	cipherText, err := aesCTRXOR(encryptKey, keyBytes, iv)
	if err != nil {
		return false, err
	}
	mac := crypto.Keccak256(derivedKey[16:32], cipherText, iv)

	// process iv, private key, salt and address to return json data and save it to path with name 'address'
	ks := KeyStoreJson{
		keyStore.Address.Hex(),
		"aes-128-ctr",
		kdfHeader,
		hex.EncodeToString(cipherText),
		hex.EncodeToString(iv),
		hex.EncodeToString(salt),
		hex.EncodeToString(mac),
		time.Now().UnixNano()/int64(time.Millisecond),
		0,
	}

	ks.StoreKey(keyStore.joinPath())
	return true, nil
}


func encrypt(key, privateKey, iv []byte) ([]byte, error) {
	aesBlock, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	stream := cipher.NewCTR(aesBlock, iv)
	cipherText := make([]byte, len(privateKey))
	stream.XORKeyStream(cipherText, privateKey)
	return cipherText, nil
}


func decrypt(key, cipherText, iv []byte) ([]byte, error) {
	aesBlock, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	stream := cipher.NewCTR(aesBlock, iv)
	privateKey := make([]byte, len(cipherText))
	stream.XORKeyStream(privateKey, cipherText)
	return privateKey, nil
}


/*
	get keystore by address and password
 */
func (keyStore *KeyStore)GetKey(auth string) error {

	// check if address exists in path or not
	filename := keyStore.joinPath()
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return err
	}

	fd, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer fd.Close()
	key := new(KeyStoreJson)

	if err := json.NewDecoder(fd).Decode(key); err != nil {
		return err
	}

	if key.Address != keyStore.Address.Hex() {
		return fmt.Errorf("key content mismatch: have address %x, want %x", key.Address, keyStore.Address.Hex())
	}

	privateKey, err := key.GetPrivateKey(auth)
	if err != nil {
		return err
	}

	keyStore.PrivateKey = *privateKey
	return nil
}


func aesCTRXOR(key, inText, iv []byte) ([]byte, error) {
	// AES-128 is selected due to size of encryptKey.
	aesBlock, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	stream := cipher.NewCTR(aesBlock, iv)
	outText := make([]byte, len(inText))
	stream.XORKeyStream(outText, inText)
	return outText, err
}


/*
	join Path and Address into a path that stores keystore
*/
func (keyStore *KeyStore) joinPath() string {
	if filepath.IsAbs(keyStore.Address.Hex()) {
		return keyStore.Address.Hex()
	}
	return filepath.Join(keyStore.Path, keyStore.Address.Hex())
}


func (keystore *KeyStoreJson)writeKeyFile(file string, content []byte) error {
	// Create the keystore directory with appropriate permissions
	// in case it is not present yet.
	const dirPerm = 0700
	if err := os.MkdirAll(filepath.Dir(file), dirPerm); err != nil {
		return err
	}
	// Atomic write: create a temporary hidden file first
	// then move it into place. TempFile assigns mode 0600.
	f, err := ioutil.TempFile(filepath.Dir(file), "."+filepath.Base(file)+".tmp")
	if err != nil {
		return err
	}
	if _, err := f.Write(content); err != nil {
		f.Close()
		os.Remove(f.Name())
		return err
	}
	f.Close()
	return os.Rename(f.Name(), file)
}
