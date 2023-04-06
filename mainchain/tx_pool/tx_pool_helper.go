package tx_pool

import (
	"io"
	"net/http"
	"strings"

	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/log"
)

// UpdateBlacklist fetch and overwrite the current blacklist
func UpdateBlacklist() error {
	httpClient := http.Client{Timeout: blacklistRequestTimeout}
	resp, err := httpClient.Get(blacklistURL)
	if err != nil {
		log.Warn("Cannot get blacklisted addresses", "err", err)
		return err
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Warn("Cannot import blacklisted addresses", "err", err)
		return err
	}
	blacklisted := strings.Split(string(body), "\n")
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
