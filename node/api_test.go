/*
 *  Copyright 2021 KardiaChain
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

package node

import (
	"bytes"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/kardiachain/go-kardia/lib/log"
	"github.com/kardiachain/go-kardia/lib/log/testlog"
	"github.com/kardiachain/go-kardia/rpc"
)

// This test uses the admin_startRPC and admin_startWS APIs,
// checking whether the HTTP server is started correctly.
func TestStartRPC(t *testing.T) {
	type test struct {
		name string
		cfg  Config
		fn   func(*testing.T, *Node, *publicAdminAPI)

		// Checks. These run after the node is configured and all API calls have been made.
		wantReachable bool // whether the HTTP server should be reachable at all
		wantHandlers  bool // whether RegisterHandler handlers should be accessible
		wantRPC       bool // whether JSON-RPC/HTTP should be accessible
		wantWS        bool // whether JSON-RPC/WS should be accessible
	}

	testCfg := testNodeConfig()
	testCfgWithHTTP := testNodeConfig()
	testCfgWithHTTP.HTTPHost = "127.0.0.1"
	testCfgWithWS := testNodeConfig()
	testCfgWithWS.WSHost = "127.0.0.1"

	tests := []test{
		{
			name: "all off",
			cfg:  *testCfg,
			fn: func(t *testing.T, n *Node, api *publicAdminAPI) {
			},
			wantReachable: false,
			wantHandlers:  false,
			wantRPC:       false,
			wantWS:        false,
		},
		// {
		// 	name: "rpc enabled through config",
		// 	cfg:  *testCfgWithHTTP,
		// 	fn: func(t *testing.T, n *Node, api *publicAdminAPI) {
		// 	},
		// 	wantReachable: true,
		// 	wantHandlers:  true,
		// 	wantRPC:       true,
		// 	wantWS:        false,
		// },
		// {
		// 	name:          "ws enabled through config",
		// 	cfg:           *testCfgWithWS,
		// 	wantReachable: true,
		// 	wantHandlers:  ,
		// 	wantRPC:       false,
		// 	wantWS:        true,
		// },
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// Apply some sane defaults.
			config := test.cfg
			config.Logger = testlog.Logger(t, log.LvlDebug)

			// Create Node.
			stack, err := New(&config)
			if err != nil {
				t.Fatal("can't create node:", err)
			}
			defer stack.Stop()

			// Register the test handler.
			stack.RegisterHandler("test", "/test", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte("OK"))
			}))

			if err := stack.Start(); err != nil {
				t.Fatal("can't start node:", err)
			}

			// Run the API call hook.
			if test.fn != nil {
				test.fn(t, stack, &publicAdminAPI{stack})
			}

			// Check if the HTTP endpoints are available.
			baseURL := stack.HTTPEndpoint()
			reachable := checkReachable(baseURL)
			handlersAvailable := checkBodyOK(baseURL + "/test")
			rpcAvailable := checkRPC(baseURL)
			wsAvailable := checkRPC(strings.Replace(baseURL, "http://", "ws://", 1))
			if reachable != test.wantReachable {
				t.Errorf("HTTP server is %sreachable, want it %sreachable", not(reachable), not(test.wantReachable))
			}
			if handlersAvailable != test.wantHandlers {
				t.Errorf("RegisterHandler handlers %savailable, want them %savailable", not(handlersAvailable), not(test.wantHandlers))
			}
			if rpcAvailable != test.wantRPC {
				t.Errorf("HTTP RPC %savailable, want it %savailable", not(rpcAvailable), not(test.wantRPC))
			}
			if wsAvailable != test.wantWS {
				t.Errorf("WS RPC %savailable, want it %savailable", not(wsAvailable), not(test.wantWS))
			}
		})
	}
}

// checkReachable checks if the TCP endpoint in rawurl is open.
func checkReachable(rawurl string) bool {
	u, err := url.Parse(rawurl)
	if err != nil {
		panic(err)
	}
	conn, err := net.Dial("tcp", u.Host)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// checkBodyOK checks whether the given HTTP URL responds with 200 OK and body "OK".
func checkBodyOK(url string) bool {
	resp, err := http.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return false
	}
	buf := make([]byte, 2)
	if _, err = io.ReadFull(resp.Body, buf); err != nil {
		return false
	}
	return bytes.Equal(buf, []byte("OK"))
}

// checkRPC checks whether JSON-RPC works against the given URL.
func checkRPC(url string) bool {
	c, err := rpc.Dial(url)
	if err != nil {
		return false
	}
	defer c.Close()

	_, err = c.SupportedModules()
	return err == nil
}

func not(ok bool) string {
	if ok {
		return ""
	}
	return "not "
}
