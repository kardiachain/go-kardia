pragma solidity ^0.5.0;
import {Ownable} from "./Ownable.sol";
import {IStaking} from "./interfaces/IStaking.sol";
import {SafeMath} from "./Safemath.sol";

contract Minter is Ownable {
    using SafeMath for uint256;
    uint256 private _oneDec = 1 * 10**18;
    uint256 public inflationRateChange = 2 * 10**16; // 2%
    uint256 public goalBonded = 20 * 10**16; // 20%;
    uint256 public blocksPerYear = 6307200; // assumption 5s per block
    uint256 public inflationMax = 7 * 10**16; // 7%
    uint256 public inflationMin = 189216 * 10**11; // 1,89216%

    uint256 public inflation;
    uint256 public annualProvision;
    uint256 public feesCollected;

    IStaking private _staking; 

    constructor() public {
        transferOwnership(msg.sender);
        _staking = IStaking(msg.sender);
    }

    // @dev mints new tokens for the previous block. Returns fee collected
    function mint() public onlyOwner returns (uint256) {
        // recalculate inflation rate
        inflation = getNextInflationRate();
        // recalculate annual provisions
        annualProvision = getNextAnnualProvisions();
        // update fee collected
        feesCollected = getBlockProvision();
        return feesCollected;
    }

    function setInflation(uint256 _inflation) public onlyOwner {
        inflation = _inflation;
    }

    function getNextAnnualProvisions() public view returns (uint256) {
        uint256 totalSupply = _staking.totalSupply();
        return inflation.mulTrun(totalSupply);
    }

    function getBlockProvision() public view returns (uint256) {
        return annualProvision.div(blocksPerYear);
    }

    function getNextInflationRate() private view returns (uint256) {
        uint256 totalBonded = _staking.totalBonded();
        uint256 totalSupply = _staking.totalSupply();
        uint256 bondedRatio = totalBonded.divTrun(totalSupply);
        uint256 inflationRateChangePerYear;
        uint256 _inflationRateChange;
        uint256 inflationRate;
        if (bondedRatio < goalBonded) {
            inflationRateChangePerYear = _oneDec
                .sub(bondedRatio.divTrun(goalBonded))
                .mulTrun(inflationRateChange);
            _inflationRateChange = inflationRateChangePerYear.div(
                blocksPerYear
            );
            inflationRate = inflation.add(_inflationRateChange);
        } else {
            inflationRateChangePerYear = bondedRatio
                .divTrun(goalBonded)
                .sub(_oneDec)
                .mulTrun(inflationRateChange);
            _inflationRateChange = inflationRateChangePerYear.div(
                blocksPerYear
            );
            if (inflation > _inflationRateChange) {
                inflationRate = inflation.sub(_inflationRateChange);
            } else {
                inflationRate = 0;
            }
        }
        if (inflationRate > inflationMax) {
            inflationRate = inflationMax;
        }
        if (inflationRate < inflationMin) {
            inflationRate = inflationMin;
        }
        return inflationRate;
    }
}