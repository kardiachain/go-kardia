package configs

import "fmt"

// SyncMode represents the synchronisation mode of the downloader.
// It is a uint32 as it is used with atomic operations.
type SyncMode uint32

const (
	FastSync SyncMode = iota // Synchronise the entire blockchain history from full blocks
	SnapSync                 // Download the chain and the state via compact snapshots
)

func (mode SyncMode) IsValid() bool {
	return mode >= FastSync && mode <= SnapSync
}

// String implements the stringer interface.
func (mode SyncMode) String() string {
	switch mode {
	case FastSync:
		return "fast"
	case SnapSync:
		return "snap"
	default:
		return "unknown"
	}
}

func (mode SyncMode) MarshalText() ([]byte, error) {
	switch mode {
	case FastSync:
		return []byte("fast"), nil
	case SnapSync:
		return []byte("snap"), nil
	default:
		return nil, fmt.Errorf("unknown sync mode %d", mode)
	}
}

func (mode *SyncMode) UnmarshalText(text []byte) error {
	switch string(text) {
	case "fast":
		*mode = FastSync
	case "snap":
		*mode = SnapSync
	default:
		return fmt.Errorf(`unknown sync mode %q, want "fast" or "snap"`, text)
	}
	return nil
}
