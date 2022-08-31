/*
   Copyright 2021 Erigon contributors

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package mdbx

import (
	"github.com/kardiachain/go-kardia/kai/storage/kvstore"
	"github.com/kardiachain/go-kardia/lib/log"

	mdbxbind "github.com/torquem-ch/mdbx-go/mdbx"
)

func MustOpen(path string) kvstore.RwDB {
	db, err := Open(path, log.New(), false)
	if err != nil {
		panic(err)
	}
	return db
}

func MustOpenRo(path string) kvstore.RoDB {
	db, err := Open(path, log.New(), true)
	if err != nil {
		panic(err)
	}
	return db
}

// Open - main method to open database.
func Open(path string, logger log.Logger, readOnly bool) (kvstore.RwDB, error) {
	var db kvstore.RwDB
	var err error
	opts := NewMDBX(logger).Path(path)
	if readOnly {
		opts = opts.Flags(func(flags uint) uint { return flags | mdbxbind.Readonly })
	}
	db, err = opts.Open()

	if err != nil {
		return nil, err
	}
	return db, nil
}
