# Test consensus with multiple nodes
Important:
  - Always include `dev` flag in test p2p. Peer address are fixed when running in dev settings.
  - Node name starts from `node1`, `node2`, etc.
  - Port number starts from `3000` for `node1`, `3001` for `node2`, and so on.
`numValid` required flag for number of validators, set to the number of nodes you plan to run.   
`genNewTxs` optional flag to routinely adds transfer transactions between genesis accounts.  
`txn` optional flag instead of `genNewTxs` to add one transfer transaction when node starts.
  
Example, 3-nodes network:  
First terminal:
```
./go-kardia --dev --numValid 3 --addr :3000 --name node1 --txn --clearDataDir
```
Second terminal:
```
./go-kardia --dev --numValid 3 --addr :3001 --name node2 --clearDataDir
```
Third terminal:
```
./go-kardia --dev --numValid 3 --addr :3002 --name node3 --clearDataDir
```

# Test consensus with bad actors
  - Simulate the voting strategy by `votingStrategy` flag in csv file `kai/dev/voting_scripts/voting_strategy_*.csv`): 
    Example:
     * 2,0,1,-1 (height=2/round=0/voteType=Prevote/bad)
     * 4,0,1,-1 (height=4/round=0/voteType=Prevote/bad)
     * 4,0,2,-1 (height=4/round=0/voteType=Precommit/bad) 
     * 5,0,1,-1 (height=5/round=0/voteType=Prevote/bad)
    
Example, 3-nodes network:  
First terminal:
```
./go-kardia --dev --numValid 3 --addr :3000 --name node1 --txn --clearDataDir --votingStrategy kai/dev/voting_scripts/voting_strategy_1.csv
```
Second terminal:
```
./go-kardia --dev --numValid 3 --addr :3001 --name node2 --clearDataDir --votingStrategy kai/dev/voting_scripts/voting_strategy_2.csv
```
Third terminal:
```
./go-kardia --dev --numValid 3 --addr :3002 --name node3 --clearDataDir --votingStrategy kai/dev/voting_scripts/voting_strategy_3.csv
``` 