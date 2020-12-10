Feature: Apply staking SMC

    Scenario: Apply staking smc success
        Given a blockchain network
        When system apply staking smc
        Then system must response with success

