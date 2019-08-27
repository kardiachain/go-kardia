package main

import (
	"github.com/kardiachain/go-kardia/configs"
	"github.com/kardiachain/go-kardia/dualchain/blockchain"
	"github.com/kardiachain/go-kardia/dualchain/service"
	"github.com/kardiachain/go-kardia/dualnode/dual_proxy"
	"github.com/kardiachain/go-kardia/dualnode/kardia"
	"github.com/kardiachain/go-kardia/kai/base"
	"github.com/kardiachain/go-kardia/lib/log"
	kai "github.com/kardiachain/go-kardia/mainchain"
)

func StartProxy(args FlagArgs, logger log.Logger, kardiaService *kai.KardiaService, dualService *service.DualService) error {
	// TODO(namdoh): Remove the hard-code below
	exchangeContractAddress := configs.GetContractAddressAt(configs.KardiaNewExchangeSmcIndex)
	exchangeContractAbi := configs.GetContractAbiByAddress(exchangeContractAddress.String())

	if args.dualChain {
		var kardiaProxy base.BlockChainAdapter
		var dualProxy base.BlockChainAdapter
		var err error

		kardiaProxy, err = kardia.NewKardiaProxy(kardiaService.BlockChain(), kardiaService.TxPool(), dualService.BlockChain(),
			dualService.EventPool(), &exchangeContractAddress, exchangeContractAbi)
		if err != nil {
			log.Error("Fail to initialize KardiaProxy", "error", err)
		}
		serviceName := ""
		if args.neoDual {
			serviceName = configs.NEO
		} else if args.ethDual {
			serviceName = configs.ETH
		} else if args.tronDual {
			serviceName = configs.TRON
		}

		if serviceName == "" {
			return nil
		}

		dualProxy, err = dual_proxy.NewProxy(
			serviceName,
			kardiaService.BlockChain(),
			kardiaService.TxPool(),
			dualService.BlockChain(),
			dualService.EventPool(),
			args.publishedEndpoint,
			args.subscribedEndpoint,
		)
		if err != nil {
			log.Error("Fail to initialize proxy", "error", err, "proxy", serviceName)
			return err
		}

		// TODO(kiendn): private and permissioned are special cases will implement them later
		//if args.isPrivateDual {
		//	// Do some validation
		//	if args.privateNodeName == "" {
		//		logger.Error("privateNodeName is required")
		//		return err
		//	}
		//	if args.privateValIndexes == "" {
		//		logger.Error("privateValIndexes is required")
		//		return err
		//	}
		//
		//	config := &permissioned.Config{
		//		Name:              &args.privateNodeName,
		//		NetworkId:         &args.privateNetworkId,
		//		ValidatorsIndices: &args.privateValIndexes,
		//		Proposal:          args.proposal,
		//		ClearData:         args.clearDataDir,
		//		ServiceName:       &args.privateServiceName,
		//		ListenAddr:        &args.privateAddr,
		//		ChainID:           &args.privateChainId,
		//	}
		//
		//	if args.serviceName != "" {
		//		config.ServiceName = &args.serviceName
		//	}
		//	if args.chainId > 0 {
		//		config.ChainID = &args.chainId
		//	}
		//	// Load address and abi of Private chain CandidateDB contract to PermissionedProxy
		//	candidateDBContractAddress, candidateDBContractAbi := configs.GetContractDetailsByIndex(configs.PrivateChainCandidateDBSmcIndex)
		//	if candidateDBContractAbi == "" {
		//		log.Error("Cannot load candidate contract abi on private chain")
		//		return err
		//	}
		//	dualProxy, err = permissioned.NewPermissionedProxy(config, kardiaService.BlockChain(),
		//		kardiaService.TxPool(), dualService.BlockChain(), dualService.EventPool(), &candidateDBContractAddress, candidateDBContractAbi)
		//	if err != nil {
		//		logger.Error("Init new private proxy failed", "error", err)
		//		return err
		//	}
		//	// Load address and abi of candidate exchange contract on Kardia to KardiaProxy
		//	candidateExchangeContractAddress, candidateExchangeContractAbi := configs.GetContractDetailsByIndex(configs.KardiaCandidateExchangeSmcIndex)
		//	if candidateExchangeContractAbi == "" {
		//		log.Error("Failed to load exchange candidate abi contract")
		//		return err
		//	}
		//	kardiaProxy, err = kardia.NewPrivateKardiaProxy(kardiaService.BlockChain(), kardiaService.TxPool(), dualService.BlockChain(),
		//		dualService.EventPool(), &candidateExchangeContractAddress, candidateExchangeContractAbi)
		//	if err != nil {
		//		log.Error("Fail to initialize PrivateKardiaProxy", "error", err)
		//	}
		//}

		// Create and pass a dual's blockchain manager to dual service, enabling dual consensus to
		// submit tx to either internal or external blockchain.
		bcManager := blockchain.NewDualBlockChainManager(kardiaProxy, dualProxy)
		dualService.SetDualBlockChainManager(bcManager)

		// Register the 'other' blockchain to each internal/external blockchain. This is needed
		// for generate Tx to submit to the other blockchain.
		kardiaProxy.RegisterExternalChain(dualProxy)
		dualProxy.RegisterInternalChain(kardiaProxy)

		dualProxy.Start()
		kardiaProxy.Start()
	}
	return nil
}
