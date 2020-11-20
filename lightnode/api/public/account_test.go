// Package public
package public

import (
	"context"
	"reflect"
	"testing"

	"github.com/kardiachain/go-kardiamain/lib/common"
	"github.com/kardiachain/go-kardiamain/lightnode"
	"github.com/kardiachain/go-kardiamain/rpc"
)

func TestAccountAPI_Balance(t *testing.T) {
	panic("not implement")
}

func TestAccountAPI_GetCode(t *testing.T) {
	type fields struct {
		service lightnode.NodeService
	}
	type args struct {
		ctx           context.Context
		address       common.Address
		blockNrOrHash rpc.BlockNumberOrHash
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    common.Bytes
		wantErr bool
	}{
		{},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &AccountAPI{
				service: tt.fields.service,
			}
			got, err := a.GetCode(tt.args.ctx, tt.args.address, tt.args.blockNrOrHash)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetCode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetCode() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAccountAPI_GetStorageAt(t *testing.T) {
	type fields struct {
		service lightnode.NodeService
	}
	type args struct {
		ctx           context.Context
		address       common.Address
		key           string
		blockNrOrHash rpc.BlockNumberOrHash
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    common.Bytes
		wantErr bool
	}{
		{},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &AccountAPI{
				service: tt.fields.service,
			}
			got, err := a.GetStorageAt(tt.args.ctx, tt.args.address, tt.args.key, tt.args.blockNrOrHash)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetStorageAt() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetStorageAt() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAccountAPI_Nonce(t *testing.T) {
	type fields struct {
		service lightnode.NodeService
	}
	type args struct {
		address string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    uint64
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &AccountAPI{
				service: tt.fields.service,
			}
			got, err := a.Nonce(tt.args.address)
			if (err != nil) != tt.wantErr {
				t.Errorf("Nonce() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Nonce() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewPublicAccountAPI(t *testing.T) {
	type args struct {
		kaiService lightnode.NodeService
	}
	tests := []struct {
		name string
		args args
		want *AccountAPI
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewPublicAccountAPI(tt.args.kaiService); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewPublicAccountAPI() = %v, want %v", got, tt.want)
			}
		})
	}
}
