package tx_pool

import (
	"strings"
	"time"

	"github.com/kardiachain/go-kardia/lib/common"
)

// UpdateBlacklist fetch and overwrite the current blacklist
func UpdateBlacklist(timeout time.Duration) error {
	// httpClient := http.Client{Timeout: timeout}
	// resp, err := httpClient.Get(blacklistURL)
	// if err != nil {
	// 	log.Warn("Cannot get blacklisted addresses", "err", err)
	// 	return err
	// }
	// body, err := io.ReadAll(resp.Body)
	// if err != nil {
	// 	log.Warn("Cannot import blacklisted addresses", "err", err)
	// 	return err
	// }
	// blacklisted := strings.Split(string(body), "\n")

	blacklisted := strings.Split("", "\n")
	for _, str := range blacklisted {
		Blacklisted[common.HexToAddress(str).Hex()] = true
	}
	return nil
}

func StringifyBlacklist() string {
	addrs := ""
	for addr, _ := range Blacklisted {
		addrs = addrs + addr + ","
	}
	return addrs
}
