# DPOS-PBFT Consensus

### Consensus Overview
An abstract description of what happens in the algorithm during the search for the N block: 


&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;NewHeight -> (Propose -> Prevote -> Precommit) -> Commit -> NewHeight ->… 


The sequence (Propose -> Prevote -> Precommit) is called a round. There may be more than one round required to commit a block at a given height.
1. `Propose` step (height:H, round:R) designated proposer proposes a block at (H,R). 
2. `Prevote` step (height:H, round:R), each validator broadcasts its prevote vote, after any +2/3 prevote received go to precommit.
3. `Precommit` step (height:H, round:R), each validator broadcasts its precommit vote,  each validator broadcasts its prevote vote, after any +2/3 prevote received go to commit.


### Test consensus with multiple nodes of different sub-groups: dual nodes, kardia validators, kardia non-validators
Important:
  - Always include `dev` flag in test p2p. Peer address are fixed when running in dev settings.
  - Node name starts from `node1`, `node2`, etc.
  - Port number starts from `3000` for `node1`, `3001` for `node2`, and so on.
  - `mainChainValIndexes` required flag to set Kardia chain validator index, e.g. set node 1 and node 2 as Kardia validators by --mainChainValIndexes 1,2
  - `dualChainValIndexes` required flag to set Dual chain validator index, e.g. set node 1 and node 2 as Dual validators by --dualChainValIndexes 1,2
  - `txn` optional flag to add one transfer transaction when node starts.
  - `logtag` optional flag, --logtag KARDIA shows log of KARDIA and --logtag DUAL shows log of DUAL 
  
Example, network of 3-Kardia-validators (node 1, 2 and 3), 2-Dual-validators (node 2 and 4), and 2-Kardia-nonvalidator(node 5 and 6):  
- Terminal 1:
```
go install; $GOPATH/bin/go-kardia --dev --addr :3000 --name node1 -txn --mainChainValIndexes 1,2,5 --dualChainValIndexes 3,4 --loglevel trace --clearDataDir --dualchain --logtag KARDIA
```
- Terminal 2:
```
go install; $GOPATH/bin/go-kardia --dev --addr :3001 --name node2 --mainChainValIndexes 1,2,5 --dualChainValIndexes 3,4 --loglevel trace --clearDataDir --dualchain --logtag KARDIA
```
- Terminal 3:
```
go install; $GOPATH/bin/go-kardia --dev --addr :3002 --name node3 --mainChainValIndexes 1,2,5 --dualChainValIndexes 3,4 --loglevel trace --clearDataDir --dualchain --logtag DUAL
```
- Terminal 4:
```
go install; $GOPATH/bin/go-kardia --dev --addr :3003 --name node4 --mainChainValIndexes 1,2,5 --dualChainValIndexes 3,4 --loglevel trace --clearDataDir --dualchain --logtag DUAL
```
- Terminal 5:
```
go install; $GOPATH/bin/go-kardia --dev --addr :3004 --name node5 --mainChainValIndexes 1,2,5 --dualChainValIndexes 3,4 --loglevel trace --clearDataDir --dualchain --logtag KARDIA
```
- Terminal 6:
```
go install; $GOPATH/bin/go-kardia --dev --addr :3005 --name node6 --mainChainValIndexes 1,2,5 --dualChainValIndexes 3,4 --loglevel trace --clearDataDir --dualchain
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
$GOPATH/bin/go-kardia --dev --addr :3000 --name node1 -txn --mainChainValIndexes 1,2,3 --clearDataDir --dualchain --logtag KARDIA --votingStrategy kai/dev/voting_scripts/voting_strategy_1.csv
```
- Terminal 2:
```
$GOPATH/bin/go-kardia --dev --addr :3001 --name node2 --mainChainValIndexes 1,2,3 --clearDataDir --dualchain --logtag KARDIA --votingStrategy kai/dev/voting_scripts/voting_strategy_2.csv
```
- Terminal 3:
```
$GOPATH/bin/go-kardia --dev --addr :3002 --name node3 --mainChainValIndexes 1,2,3 --clearDataDir --dualchain --logtag KARDIA --votingStrategy kai/dev/voting_scripts/voting_strategy_3.csv
``` 
