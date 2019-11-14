package main

type (
	Config struct {
		Node                 `yaml:"Node"`
		MainChain   *Chain   `yaml:"MainChain"`
		DualChain   *Chain   `yaml:"DualChain,omitempty"`
	}
	Node struct {
		P2P                        `yaml:"P2P"`
		LogLevel          string   `yaml:"LogLevel"`
		Name              string   `yaml:"Name"`
		DataDir           string   `yaml:"DataDir"`
		HTTPHost          string   `yaml:"HTTPHost"`
		HTTPPort          int      `yaml:"HTTPPort"`
		HTTPModules       []string `yaml:"HTTPModules"`
		HTTPVirtualHosts  []string `yaml:"HTTPVirtualHosts"`
		HTTPCors          []string `yaml:"HTTPCors"`
	}
	P2P struct {
		PrivateKey    string    `yaml:"PrivateKey"`
		ListenAddress string    `yaml:"ListenAddress"`
		MaxPeers      int       `yaml:"MaxPeers"`
	}
	Chain struct {
		ServiceName   string         `yaml:"ServiceName"`
		Protocol      *string        `yaml:"Protocol,omitempty"`
		ChainID       uint64         `yaml:"ChainID"`
		NetworkID     uint64         `yaml:"NetworkID"`
		AcceptTxs     uint32         `yaml:"AcceptTxs"`
		ZeroFee       uint           `yaml:"ZeroFee"`
		IsDual        uint           `yaml:"IsDual"`
		Consensus     *Consensus      `yaml:"Consensus,omitempty"`
		Genesis       *Genesis       `yaml:"Genesis,omitempty"`
		TxPool        *Pool          `yaml:"TxPool,omitempty"`
		EventPool     *Pool          `yaml:"EventPool,omitempty"`
		Database      *Database      `yaml:"Database,omitempty"`
	   	Seeds         []string       `yaml:"Seeds"`
		Events        []Event 	     `yaml:"Events"`
		PublishedEndpoint  *string   `yaml:"PublishedEndpoint,omitempty"`
		SubscribedEndpoint *string   `yaml:"SubscribedEndpoint,omitempty"`
		Validators    []int          `yaml:"Validators,omitempty"`
		BaseAccount   BaseAccount    `yaml:"BaseAccount"`
	}
	Genesis struct {
		Addresses      []string      `yaml:"Addresses"`
		GenesisAmount  string        `yaml:"GenesisAmount"`
		Contracts      []Contract    `yaml:"Contracts"`
	}
	Consensus struct {
		MaxValidators       uint64            `yaml:"MaxValidators"`
		ConsensusPeriod     uint64            `yaml:"ConsensusPeriod"`
		MinimumStakes       string            `yaml:"MinimumStakes"`
		Master     			Contract          `yaml:"Master"`
		Stakers    			NodesOrStakes     `yaml:"Stakers"`
		Nodes      			NodesOrStakes     `yaml:"Nodes"`
	}
	NodeInfo struct {
		Address    string       `yaml:"Address"`
		Owner      string       `yaml:"Owner"`
		PubKey     string       `yaml:"PubKey"`
		Name       string       `yaml:"Name"`
		Host       string       `yaml:"Host"`
		Port       string       `yaml:"Port"`
		Reward     uint16       `yaml:"Reward"`
	}
	StakerInfo struct {
		Address     string       `yaml:"Address"`
		Owner       string       `yaml:"Owner"`
		StakedNode  string       `yaml:"StakedNode"`
		LockedPeriod uint64      `yaml:"LockedPeriod"`
		StakeAmount string       `yaml:"StakeAmount"`
	}
	NodesOrStakes struct {
		ByteCode     string       `yaml:"ByteCode"`
		ABI          string       `yaml:"ABI"`
		NodeInfo     []NodeInfo   `yaml:"NodeInfo,omitempty"`
		StakerInfo   []StakerInfo `yaml:"StakerInfo,omitempty"`
	}
	Contract struct {
		Address    string    `yaml:"Address,omitempty"`
		ByteCode   string    `yaml:"ByteCode"`
		ABI        string    `yaml:"ABI,omitempty"`
		GenesisAmount string `yaml:"GenesisAmount,omitempty"`
	}
	Pool struct {
		GlobalSlots     uint64    `yaml:"GlobalSlots"`
		GlobalQueue     uint64    `yaml:"GlobalQueue"`
		NumberOfWorkers int       `yaml:"NumberOfWorkers"`
		WorkerCap       int       `yaml:"WorkerCap"`
		BlockSize       int       `yaml:"BlockSize"`
	}
	Database struct {
		Type         uint      `yaml:"Type"`
		Dir          string    `yaml:"Dir"`
		Caches       int       `yaml:"Caches"`
		Handles      int       `yaml:"Handles"`
		URI          string    `yaml:"URI"`
		Name         string    `yaml:"Name"`
		Drop         int       `yaml:"Drop"`
	}
	Event struct {
		MasterSmartContract string           `yaml:"MasterSmartContract"`
		ContractAddress   string             `yaml:"ContractAddress"`
		MasterABI         *string            `yaml:"MasterABI"`
		ABI               *string            `yaml:"ABI,omitempty"`
		Watchers          []Watcher          `yaml:"Watchers"`
	}
	Watcher struct {
		Method           string             `yaml:"Method"`
		WatcherActions   []string           `yaml:"WatcherActions,omitempty"`
		DualActions      []string           `yaml:"DualActions"`
	}
	BaseAccount struct {
		Address      string       `yaml:"Address"`
		PrivateKey   string       `yaml:"PrivateKey"`
	}
)
