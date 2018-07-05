package kai

// DefaultConfig contains default settings for use on the Kardia main net.
var DefaultConfig = Config{

	NetworkId: 1,
}

//go:generate gencodec -type Config -field-override configMarshaling -formats toml -out gen_config.go

type Config struct {
	// Protocol options
	NetworkId uint64 // Network
}
