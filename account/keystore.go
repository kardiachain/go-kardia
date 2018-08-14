package account

import (
	"crypto/aes"
	"crypto/ecdsa"
	"go-kardia/lib/crypto"
	"crypto/rand"
	"encoding/hex"
	"golang.org/x/crypto/scrypt"
	"go-kardia/lib/common"
	"crypto/cipher"
	"time"
	"os"
	"path/filepath"
	"io/ioutil"
	"encoding/json"
	"fmt"
	"io"
	"errors"
	"go-kardia/types"
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
	Address common.Address
	PrivateKey ecdsa.PrivateKey
}


/*
	Create new keystore based on path, password
 */
func (keyStore *KeyStore)createKeyStore(auth string) (bool, error) {
	// Convert auth (password) to byte array
	authArray := []byte(auth)

	// Get random iv
	iv, err := GetRandomBytes(aes.BlockSize)
	if err != nil {
		return false, err
	}

	// Get random salt
	salt, err := GetRandomBytes(scryptDKLen)
	if err != nil {
		return false, err
	}

	// Get random private key
	privateKey, err := ecdsa.GenerateKey(crypto.S256(), rand.Reader)
	if err != nil {
		return false, err
	}
	// Get address from private key
	keyStore.PrivateKey = *privateKey
	keyStore.Address = common.Address(crypto.PubkeyToAddress(privateKey.PublicKey))

	// Derived key
	derivedKey, err := scrypt.Key(authArray, salt, scryptN, scryptR, scryptP, scryptDKLen)
	if err != nil {
		return false, err
	}

	// Generate encrypted key, cipher text and mac
	encryptKey := derivedKey[:16]
	keyBytes := common.PaddedBigBytes(privateKey.D, 32)
	cipherText, err := aesCTRXOR(encryptKey, keyBytes, iv)
	if err != nil {
		return false, err
	}
	mac := crypto.Keccak256(derivedKey[16:32], cipherText, iv)

	// Add iv, private key, salt and address to KeyStoreJson and save it to path with name 'address'
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


/*
	Create a random byte array based on input len
 */
func GetRandomBytes(len int) ([]byte, error){
	value := make([]byte, len)
	if _, err := io.ReadFull(rand.Reader, value); err != nil {
		return nil, errors.New("reading from crypto/rand failed: " + err.Error())
	}

	return value, nil
}


/*
	Get keystore by password
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

	privateKey, err := GetKeyFromJSON(key, auth)
	if err != nil {
		return err
	}

	keyStore.PrivateKey = *privateKey
	return nil
}


/*
	Get PrivateKey from KeyStoreJSON
	This function is used for testing case or cases that there aren't any keystores stored in local storage
 */
func GetKeyFromJSON(jsonData *KeyStoreJson, auth string) (*ecdsa.PrivateKey, error) {

	if jsonData == nil {
		return nil, errors.New("jsonData is empty")
	}

	return jsonData.GetPrivateKey(auth)
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


/*
	Sign a transaction with current keystore
	TODO(kiendn@): Should we implement lock/unlock account?
 */
func (keyStore *KeyStore) SignTransaction(transaction *types.Transaction) (*types.Transaction, error) {
	return types.SignTx(transaction, &keyStore.PrivateKey)
}

