package main

type (
	Config struct {
		Node      `yaml:"Node"`
		MainChain *Chain `yaml:"MainChain"`
		DualChain *Chain `yaml:"DualChain,omitempty"`
	}
	Node struct {
		P2P              `yaml:"P2P"`
		LogLevel         string   `yaml:"LogLevel"`
		Name             string   `yaml:"Name"`
		DataDir          string   `yaml:"DataDir"`
		HTTPHost         string   `yaml:"HTTPHost"`
		HTTPPort         int      `yaml:"HTTPPort"`
		HTTPModules      []string `yaml:"HTTPModules"`
		HTTPVirtualHosts []string `yaml:"HTTPVirtualHosts"`
		HTTPCors         []string `yaml:"HTTPCors"`
	}
	P2P struct {
		PrivateKey    string `yaml:"PrivateKey"`
		ListenAddress string `yaml:"ListenAddress"`
		MaxPeers      int    `yaml:"MaxPeers"`
	}
	Chain struct {
		ServiceName        string      `yaml:"ServiceName"`
		Protocol           *string     `yaml:"Protocol,omitempty"`
		ChainID            uint64      `yaml:"ChainID"`
		NetworkID          uint64      `yaml:"NetworkID"`
		AcceptTxs          uint32      `yaml:"AcceptTxs"`
		ZeroFee            uint        `yaml:"ZeroFee"`
		IsDual             uint        `yaml:"IsDual"`
		Genesis            *Genesis    `yaml:"Genesis,omitempty"`
		TxPool             *Pool       `yaml:"TxPool,omitempty"`
		EventPool          *Pool       `yaml:"EventPool,omitempty"`
		Database           *Database   `yaml:"Database,omitempty"`
		Seeds              []string    `yaml:"Seeds"`
		Events             []Event     `yaml:"Events"`
		PublishedEndpoint  *string     `yaml:"PublishedEndpoint,omitempty"`
		SubscribedEndpoint *string     `yaml:"SubscribedEndpoint,omitempty"`
		Validators         []int       `yaml:"Validators"`
		BaseAccount        BaseAccount `yaml:"BaseAccount"`
	}
	Genesis struct {
		Addresses     []string   `yaml:"Addresses"`
		GenesisAmount string     `yaml:"GenesisAmount"`
		Contracts     []Contract `yaml:"Contracts"`
	}
	Contract struct {
		Address  string `yaml:"Address"`
		ByteCode string `yaml:"ByteCode"`
		ABI      string `yaml:"ABI,omitempty"`
	}
	Pool struct {
		GlobalSlots      uint64 `yaml:"GlobalSlots"`
		GlobalQueue      uint64 `yaml:"GlobalQueue"`
		NumberOfWorkers  int    `yaml:"NumberOfWorkers"`
		WorkerCap        int    `yaml:"WorkerCap"`
		BlockSize        int    `yaml:"BlockSize"`
		//BlockSizePercent uint64 `yaml:"BlockSizePercent"`
		LifeTime         int    `yaml:"LifeTime"`
	}
	Database struct {
		Type    uint   `yaml:"Type"`
		Dir     string `yaml:"Dir"`
		Caches  int    `yaml:"Caches"`
		Handles int    `yaml:"Handles"`
		URI     string `yaml:"URI"`
		Name    string `yaml:"Name"`
		Drop    int    `yaml:"Drop"`
	}
	Event struct {
		ContractAddress string          `yaml:"ContractAddress"`
		ABI             *string         `yaml:"ABI,omitempty"`
		WatcherActions  []WatcherAction `yaml:"WatcherActions"`
		DualActions     []string        `yaml:"DualActions"`
	}
	WatcherAction struct {
		Method     string `yaml:"Method"`
		DualAction string `yaml:"DualAction"`
	}
	BaseAccount struct {
		Address    string `yaml:"Address"`
		PrivateKey string `yaml:"PrivateKey"`
	}
)
