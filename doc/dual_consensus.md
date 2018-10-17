# Dual Consensus (previously known as Group Consensus)

* Upon seeing an expected event from Ether. Dual's listening service:
  - extracts relevant bits, called eventInfo, from the Ether's event that are needed
    to generate a Kardia's transaction, called kardiaTx.
  - creates a DualEvent, containing the Ether event's hash (called ethTxHash),
    eventInfo, kardiaTx's hash (called kardiaTxHash). Why storing such info?
    - ethTxHash: to verify such event exists
    - eventInfo: so other dual validators can re-compute their own Kardia's
      transactions to verify the kardiaTx proposed by a dual proposer
    - kardiaTxHash: to inform other dual's peers that a tx with this hash will
      be submitted to Kardia's EventPool by the next proposer.
  - adds the DualEvent to dual's EventPool, which will eventually selected by a dual
    node proposer when preparing a new dual's block.
  - stores both ethTxHash and kardiaTxHash in local DB to validate against the
    future proposed dual's block.

* Dual node proposer would dequeue a DualEvent from dual's EventPool, add to
  a proposing block, and then broadcast it to all other peers.

* Dual node validator, upon receiving the proposed block:
  - validates its DualEvent's ethTxHash against what it has seen (since all dual
    nodes must have seen the same expected Ether's event)
  - generates a kardiaTx from the proposed block's eventInfo and verify its hash
    against the proposal's block kardiaTxHash.
  - performs other standard validation checks.

  Note: Although it's easier to compare the proposed block's DualEvent against the
  validator node's EventPool. There is no guarantee that the DualEvent is still in
  the pool since it may be already removed from the pool (e.g. too old?).

  Note: To support multi-sig, each node would sign on the kardiaTxHash after its
  own validation, the signature can be stored in Vote and will be eventually
  Broadcasted back to the proposer, following standard consensus flow.

* When the next dual proposer proposes a new block at the next height, it would look 
  at all the DualEvent in the previous block, compute kardiaTx, and submit it (along
  With the multi-sig collected from previous Votes) to Kardia's TxPool.
  
* Data structures (note that these fields map loosely to the field described above):
  // A building "block" of dual's service.
  type DualBlock struct {
  	DualEvents []*types.DualEvent
  	...
  }
  
  // An event pertaining to the current dual node's interests and its derived tx's
  // metadata.
  type DualEvent struct {
  	TriggeredEvent EventData 
  	PendingTx      TxData
  }
  
  // Data relevant to the event (either from external or internal blockchain)
  // that pertains to the current dual node's interests.
  type EventData struct {
	TxHash common.Hash
	Source string
	Data   EventSummary
  }
  
  // Relevant bits for necessary for computing internal tx (ie. Kardia's tx)
  // or external tx (ie. Ether's tx, Neo's tx).
  type EventSummary struct {
  	...
  }
  
  // Metadata relevant to the tx that will be submit to other blockchain (internally
  // or externally).
  type TxData struct {
  	TxHash common.Hash
  	Target string
  }