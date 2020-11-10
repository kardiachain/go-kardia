// Package _default
package _default

import (
	typesCfg "github.com/kardiachain/go-kardiamain/configs/types"
	kaiproto "github.com/kardiachain/go-kardiamain/proto/kardiachain/types"
)

var (
	validators = []*typesCfg.Validator{
		{
			Name:    "val1",
			Address: "0xc1fe56E3F58D3244F606306611a5d10c8333f1f6",
			Power:   10000000000000000,
		},
		{
			Name:    "val2",
			Address: "0x7cefC13B6E2aedEeDFB7Cb6c32457240746BAEe5",
			Power:   10000000000000000,
		},
		{
			Name:    "val3",
			Address: "0xfF3dac4f04dDbD24dE5D6039F90596F0a8bb08fd",
			Power:   10000000000000000,
		},
	}
)

func Validators() []*typesCfg.Validator {
	return validators
}

func ValidatorParams() kaiproto.ValidatorParams {
	return kaiproto.ValidatorParams{}
}
