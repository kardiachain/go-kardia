pragma solidity ^0.5.0;
import {Ownable} from "./Ownable.sol";
import {IStaking} from "./interfaces/IStaking.sol";
import {SafeMath} from "./Safemath.sol";
import {IParams} from "./interfaces/IParams.sol";

contract Minter is Ownable{
    using SafeMath for uint256;
    uint256 private _oneDec = 1 * 10**18;
    uint256 public inflation;
    uint256 public annualProvision;
    uint256 public feesCollected;

    IStaking private _staking;
    address public params;

    constructor(address _params) public {
        params = _params;
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

    function setAnnualProvision(uint256 _annualProvision) public onlyOwner {
        annualProvision = _annualProvision;
    }

    function getNextAnnualProvisions() public view returns (uint256) {
        uint256 totalSupply = _staking.totalSupply();
        return inflation.mulTrun(totalSupply);
    }

    function getBlockProvision() public view returns (uint256) {
        return annualProvision.div(IParams(params).getBlocksPerYear());
    }

    function getNextInflationRate() private view returns (uint256) {
        uint256 totalBonded = _staking.totalBonded();
        uint256 totalSupply = _staking.totalSupply();
        uint256 bondedRatio = totalBonded.divTrun(totalSupply);
        uint256 inflationRateChangePerYear;
        uint256 _inflationRateChange;
        uint256 inflationRate;
        if (bondedRatio < IParams(params).getGoalBonded()) {
            inflationRateChangePerYear = _oneDec
                .sub(bondedRatio.divTrun(IParams(params).getGoalBonded()))
                .mulTrun(IParams(params).getInflationRateChange());
            _inflationRateChange = inflationRateChangePerYear.div(
               IParams(params).getBlocksPerYear()
            );
            inflationRate = inflation.add(_inflationRateChange);
        } else {
            inflationRateChangePerYear = bondedRatio
                .divTrun(IParams(params).getGoalBonded())
                .sub(_oneDec)
                .mulTrun(IParams(params).getInflationRateChange());
            _inflationRateChange = inflationRateChangePerYear.div(
                IParams(params).getBlocksPerYear()
            );
            if (inflation > _inflationRateChange) {
                inflationRate = inflation.sub(_inflationRateChange);
            } else {
                inflationRate = 0;
            }
        }
        if (inflationRate > IParams(params).getInflationMax()) {
            inflationRate = IParams(params).getInflationMax();
        }
        if (inflationRate < IParams(params).getInflationMin()) {
            inflationRate = IParams(params).getInflationMin();
        }
        return inflationRate;
    }
}