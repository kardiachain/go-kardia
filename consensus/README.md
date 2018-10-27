# DPOS-PBFT Consensus

### Test consensus with multiple nodes of different sub-groups: dual nodes, kardia validators, kardia non-validators
Important:
  - Always include `dev` flag in test p2p. Peer address are fixed when running in dev settings.
  - Node name starts from `node1`, `node2`, etc.
  - Port number starts from `3000` for `node1`, `3001` for `node2`, and so on.
  - `mainChainValIndex` required flag to set Kardia chain validator index, e.g. set node2 as Kardia validator by --mainChainValIndex 2
  - `dualChainValIndex` required flag to set Dual chain validator index, e.g. set node2 as Dual validator by --dualChainValIndex 2
  - `txn` optional flag to add one transfer transaction when node starts.
  - `logtag` optional flag, --logtag KARDIA shows log of KARDIA and --logtag DUAL shows log of DUAL 
  
Example, network of 3-Kardia-validators (node 1, 2 and 3), 2-Dual-validators (node 2 and 4), and 2-Kardia-nonvalidator(node 5 and 6):  
- Terminal 1:
```
$GOPATH/bin/go-kardia --dev --addr :3000 --name node1 -txn --mainChainValIndex 2 --dualChainValIndex 2 --mainChainValIndex 3 --mainChainValIndex 1 --dualChainValIndex 4 --clearDataDir --dualchain --logtag KARDIA
```
- Terminal 2:
```
$GOPATH/bin/go-kardia --dev --addr :3001 --name node2 --mainChainValIndex 2 --dualChainValIndex 2 --mainChainValIndex 3 --mainChainValIndex 1 --dualChainValIndex 4 --clearDataDir --dualchain --logtag DUAL
```
- Terminal 3:
```
$GOPATH/bin/go-kardia --dev --addr :3002 --name node3 --mainChainValIndex 2 --dualChainValIndex 2 --mainChainValIndex 3 --mainChainValIndex 1 --dualChainValIndex 4 --clearDataDir --dualchain --logtag KARDIA
```
- Terminal 4:
```
$GOPATH/bin/go-kardia --dev --addr :3003 --name node4 --mainChainValIndex 2 --dualChainValIndex 2 --mainChainValIndex 3 --mainChainValIndex 1 --dualChainValIndex 4 --clearDataDir --dualchain --logtag DUAL
```
- Terminal 5:
```
$GOPATH/bin/go-kardia --dev --addr :3004 --name node5 --mainChainValIndex 2 --dualChainValIndex 2 --mainChainValIndex 3 --mainChainValIndex 1 --dualChainValIndex 4 --clearDataDir --dualchain
```
- Terminal 6:
```
$GOPATH/bin/go-kardia --dev --addr :3005 --name node6 --mainChainValIndex 2 --dualChainValIndex 2 --mainChainValIndex 3 --mainChainValIndex 1 --dualChainValIndex 4 --clearDataDir --dualchain
```

### Test consensus with bad actors
  - Simulate the voting strategy by `votingStrategy` flag in csv file `kai/dev/voting_scripts/voting_strategy_*.csv`): 
    Example:
     * 2,0,1,-1 (height=2/round=0/voteType=Prevote/bad)
     * 4,0,1,-1 (height=4/round=0/voteType=Prevote/bad)
     * 4,0,2,-1 (height=4/round=0/voteType=Precommit/bad) 
     * 5,0,1,-1 (height=5/round=0/voteType=Prevote/bad)
    
Example, 3-nodes network:  
- Terminal 1:
```
$GOPATH/bin/go-kardia --dev --addr :3000 --name node1 -txn --mainChainValIndex 2  --mainChainValIndex 3 --mainChainValIndex 1 --clearDataDir --dualchain --logtag KARDIA --votingStrategy kai/dev/voting_scripts/voting_strategy_1.csv
```
- Terminal 2:
```
$GOPATH/bin/go-kardia --dev --addr :3001 --name node2 --mainChainValIndex 2  --mainChainValIndex 3 --mainChainValIndex 1 --clearDataDir --dualchain --logtag KARDIA --votingStrategy kai/dev/voting_scripts/voting_strategy_2.csv
```
- Terminal 3:
```
$GOPATH/bin/go-kardia --dev --addr :3002 --name node3 --mainChainValIndex 2  --mainChainValIndex 3 --mainChainValIndex 1 --clearDataDir --dualchain --logtag KARDIA --votingStrategy kai/dev/voting_scripts/voting_strategy_3.csv
``` 
