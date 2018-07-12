package kai

import "time"

const (
	forceSyncCycle      = 10 * time.Second // Time interval to force syncs
	minDesiredPeerCount = 5                // Amount of peers desired to start syncing

)

// syncer is responsible for periodically synchronising with the network, both
// downloading hashes and blocks as well as handling the announcement handler.
func syncNetwork(pm *ProtocolManager) {
	// TODO: Start and ensure cleanup of sync mechanisms

	// Wait for different events to fire synchronisation operations
	forceSync := time.NewTicker(forceSyncCycle)
	defer forceSync.Stop()

	for {
		select {
		case <-pm.newPeerCh:
			// Make sure we have peers to select from, then sync
			if pm.peers.Len() < minDesiredPeerCount {
				break
			}
			// TODO: sync blockchain

		case <-forceSync.C:
			// Force a sync even if not enough peers are present
			// TODO: sync blockchain

		case <-pm.noMorePeers:
			return
		}
	}
}
