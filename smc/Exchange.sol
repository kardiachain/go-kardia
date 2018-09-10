pragma solidity ^0.4.24;
contract Demo {
    uint256 totalEth;
    uint256 totalNeo;

    function matchEth(uint256 eth) public {
        totalEth += eth;
    }
    function matchNeo(uint256 neo) public {
        totalNeo += neo;
    }
    function getEthToSend() public view returns (uint256) {
        if (totalNeo > totalEth) {
            return totalEth;
        }
        return totalNeo;
    }
    function getNeoToSend() public view returns (uint256) {
        if (totalEth > totalNeo) {
            return totalNeo;
        }
        return totalEth;
    }
    function removeEth(uint256 eth) public {
        require(totalEth >= eth);
        totalEth -= eth;
    }
    function removeNeo(uint256 neo) public {
        require(totalNeo >= neo);
        totalNeo -= neo;
    }
}