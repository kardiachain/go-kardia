package p2p

import (
	"crypto/ecdsa"
	"fmt"
	golog "log"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kardiachain/go-kardiamain/configs"
	"github.com/kardiachain/go-kardiamain/lib/bytes"
	"github.com/kardiachain/go-kardiamain/lib/crypto"
	"github.com/kardiachain/go-kardiamain/lib/log"

	kconn "github.com/kardiachain/go-kardiamain/lib/p2p/conn"
)

func TestPeerBasic(t *testing.T) {
	assert, require := assert.New(t), require.New(t)
	priv1, _ := crypto.GenerateKey()
	// simulate remote peer
	rp := &remotePeer{PrivKey: priv1, Config: cfg}
	rp.Start()
	t.Cleanup(rp.Stop)

	p, err := createOutboundPeerAndPerformHandshake(rp.Addr(), cfg, kconn.DefaulKAIConnConfig())
	require.Nil(err)

	err = p.Start()
	require.Nil(err)
	t.Cleanup(func() {
		if err := p.Stop(); err != nil {
			t.Error(err)
		}
	})

	assert.True(p.IsRunning())
	assert.True(p.IsOutbound())
	assert.False(p.IsPersistent())
	p.persistent = true
	assert.True(p.IsPersistent())
	assert.Equal(rp.Addr().DialString(), p.RemoteAddr().String())
	assert.Equal(rp.ID(), p.ID())
}

func TestPeerSend(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	config := cfg
	priv1, _ := crypto.GenerateKey()
	// simulate remote peer
	rp := &remotePeer{PrivKey: priv1, Config: config}
	rp.Start()
	t.Cleanup(rp.Stop)

	p, err := createOutboundPeerAndPerformHandshake(rp.Addr(), config, kconn.DefaulKAIConnConfig())
	require.Nil(err)

	err = p.Start()
	require.Nil(err)

	t.Cleanup(func() {
		if err := p.Stop(); err != nil {
			t.Error(err)
		}
	})

	assert.True(p.CanSend(testCh))
	assert.True(p.Send(testCh, []byte("Asylum")))
}

func createOutboundPeerAndPerformHandshake(
	addr *NetAddress,
	config *configs.P2PConfig,
	mConfig kconn.MConnConfig,
) (*peer, error) {
	chDescs := []*kconn.ChannelDescriptor{
		{ID: testCh, Priority: 1},
	}
	reactorsByCh := map[byte]Reactor{testCh: NewTestReactor(chDescs, true)}
	pk, err := crypto.GenerateKey()
	pc, err := testOutboundPeerConn(addr, config, false, pk)
	if err != nil {
		return nil, err
	}
	timeout := 1 * time.Second
	ourNodeInfo := testNodeInfo(addr.ID, "host_peer")
	peerNodeInfo, err := handshake(pc.conn, timeout, ourNodeInfo)
	if err != nil {
		return nil, err
	}

	p := newPeer(pc, mConfig, peerNodeInfo, reactorsByCh, chDescs, func(p Peer, r interface{}) {})
	p.SetLogger(log.TestingLogger())
	return p, nil
}

func testDial(addr *NetAddress, cfg *configs.P2PConfig) (net.Conn, error) {
	if cfg.TestDialFail {
		return nil, fmt.Errorf("dial err (peerConfig.DialFail == true)")
	}

	conn, err := addr.DialTimeout(cfg.DialTimeout)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func testOutboundPeerConn(
	addr *NetAddress,
	config *configs.P2PConfig,
	persistent bool,
	ourNodePrivKey *ecdsa.PrivateKey,
) (peerConn, error) {

	var pc peerConn
	conn, err := testDial(addr, config)
	if err != nil {
		return pc, fmt.Errorf("error creating peer: %w", err)
	}

	pc, err = testPeerConn(conn, config, true, persistent, ourNodePrivKey, addr)
	if err != nil {
		if cerr := conn.Close(); cerr != nil {
			return pc, fmt.Errorf("%v: %w", cerr.Error(), err)
		}
		return pc, err
	}

	// ensure dialed ID matches connection ID
	if addr.ID != pc.ID() {
		if cerr := conn.Close(); cerr != nil {
			return pc, fmt.Errorf("%v: %w", cerr.Error(), err)
		}
		return pc, ErrSwitchAuthenticationFailure{addr, pc.ID()}
	}

	return pc, nil
}

type remotePeer struct {
	PrivKey    *ecdsa.PrivateKey
	Config     *configs.P2PConfig
	addr       *NetAddress
	channels   bytes.HexBytes
	listenAddr string
	listener   net.Listener
}

func (rp *remotePeer) Addr() *NetAddress {
	return rp.addr
}

func (rp *remotePeer) ID() ID {
	return PubKeyToID(rp.PrivKey.PublicKey)
}

func (rp *remotePeer) Start() {
	if rp.listenAddr == "" {
		rp.listenAddr = "127.0.0.1:0"
	}

	l, e := net.Listen("tcp", rp.listenAddr) // any available address
	if e != nil {
		golog.Fatalf("net.Listen tcp :0: %+v", e)
	}
	rp.listener = l
	rp.addr = NewNetAddress(PubKeyToID(rp.PrivKey.PublicKey), l.Addr())
	if rp.channels == nil {
		rp.channels = []byte{testCh}
	}
	go rp.accept()
}

func (rp *remotePeer) Stop() {
	rp.listener.Close()
}

func (rp *remotePeer) Dial(addr *NetAddress) (net.Conn, error) {
	conn, err := addr.DialTimeout(1 * time.Second)
	if err != nil {
		return nil, err
	}
	pc, err := testInboundPeerConn(conn, rp.Config, rp.PrivKey)
	if err != nil {
		return nil, err
	}
	_, err = handshake(pc.conn, time.Second, rp.nodeInfo())
	if err != nil {
		return nil, err
	}
	return conn, err
}

func (rp *remotePeer) accept() {
	conns := []net.Conn{}

	for {
		conn, err := rp.listener.Accept()
		if err != nil {
			golog.Printf("Failed to accept conn: %+v", err)
			for _, conn := range conns {
				_ = conn.Close()
			}
			return
		}

		pc, err := testInboundPeerConn(conn, rp.Config, rp.PrivKey)
		if err != nil {
			golog.Fatalf("Failed to create a peer: %+v", err)
		}

		_, err = handshake(pc.conn, time.Second, rp.nodeInfo())
		if err != nil {
			golog.Fatalf("Failed to perform handshake: %+v", err)
		}

		conns = append(conns, conn)
	}
}

func (rp *remotePeer) nodeInfo() NodeInfo {
	return DefaultNodeInfo{
		ProtocolVersion: defaultProtocolVersion,
		DefaultNodeID:   rp.Addr().ID,
		ListenAddr:      rp.listener.Addr().String(),
		Network:         "testing",
		Version:         "1.2.3-rc0-deadbeef",
		Channels:        rp.channels,
		Moniker:         "remote_peer",
	}
}
