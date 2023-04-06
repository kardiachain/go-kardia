package tx_pool

import (
	"io"
	"net/http"
	"strings"

	"github.com/kardiachain/go-kardia/lib/common"
	"github.com/kardiachain/go-kardia/lib/log"
)

var UpdateBlacklistInterval uint64 = 50 // blocks since last update

func UpdateBlacklist() error {
	resp, err := http.Get("https://raw.githubusercontent.com/kardiachain/consensus/main/notes")
	if err != nil {
		log.Crit("Cannot get blacklisted addresses", "err", err)
		return err
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Crit("Cannot import blacklisted addresses", "err", err)
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
