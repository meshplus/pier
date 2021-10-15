package main

import "github.com/urfave/cli"

var (
	bxhAddrFlag = cli.StringSliceFlag{
		Name:     "addr",
		Usage:    "Specific bitxhub node address",
		Required: false,
	}
	adminKeyPathFlag = cli.StringFlag{
		Name:     "admin-key",
		Usage:    "Specific admin key path",
		Required: true,
	}
	//methodFlag = cli.StringFlag{
	//	Name:     "method",
	//	Usage:    "Specific did sub method name(like appchain)",
	//	Required: true,
	//}
	//didFlag = cli.StringFlag{
	//	Name:     "did",
	//	Usage:    "Specific full did name(like did:bitxhub:appchain1:0xc7F999b83Af6DF9e67d0a37Ee7e900bF38b3D013)",
	//	Required: true,
	//}
	statusFlag = cli.IntFlag{
		Name:     "status",
		Usage:    "Specify the status you want to set(1 is pass, 0 is reject, default is 1)",
		Required: false,
		Value:    1,
	}
	//didDocAddrFlag = cli.StringFlag{
	//	Name:     "doc-addr",
	//	Usage:    "Specify the addr of did document",
	//	Required: true,
	//}
	//didDocHashFlag = cli.StringFlag{
	//	Name:     "doc-hash",
	//	Usage:    "Specify the hash of did document",
	//	Required: true,
	//}

	// appchain info related flags
	appchainIdFlag = cli.StringFlag{
		Name:     "appchain-id",
		Usage:    "Specify appchain id",
		Required: true,
	}

	appchainNameFlag = cli.StringFlag{
		Name:     "name",
		Usage:    "Specify appchain name",
		Required: true,
	}
	appchainTypeFlag = cli.StringFlag{
		Name:     "type",
		Usage:    "Specify appchain type",
		Required: true,
	}
	appchainDescFlag = cli.StringFlag{
		Name:     "desc",
		Usage:    "Specify appchain description",
		Required: true,
	}
	appchainVersionFlag = cli.StringFlag{
		Name:     "version",
		Usage:    "Specify appchain version",
		Required: true,
	}
	appchainValidatorFlag = cli.StringFlag{
		Name:     "validators",
		Usage:    "Specific appchain validators path",
		Required: true,
	}
	appchainTrustRootFlag = cli.StringFlag{
		Name:     "trustroot",
		Usage:    "Specify appchain trustroot path",
		Required: true,
	}
	appchainBrokerFlag = cli.StringFlag{
		Name:     "broker",
		Usage:    "Specify appchain broker contract address",
		Required: false,
	}
	fabricBrokerChannelIDFlag = cli.StringFlag{
		Name:     "broker-cid",
		Usage:    "Specify fabric broker contract channel ID, only for fabric appchain",
		Required: false,
	}
	fabricBrokerChaincodeIDFlag = cli.StringFlag{
		Name:     "broker-ccid",
		Usage:    "Specify fabric broker contract chaincode ID, only for fabric appchain",
		Required: false,
	}
	fabricBrokerVersionFlag = cli.StringFlag{
		Name:     "broker-v",
		Usage:    "Specify appchain broker contract version, only for fabric appchain",
		Required: false,
	}
	appchainBindFlag = cli.StringSliceFlag{
		Name:     "bind",
		Usage:    "Specify if bind default rule(for fabric 1.4 appchain, true or false)",
		Required: true,
	}
	appchainMasterRuleFlag = cli.StringFlag{
		Name:     "master-rule",
		Usage:    "Specify appchain master-rule",
		Required: true,
	}
	appchainMasterRuleUrlFlag = cli.StringFlag{
		Name:     "rule-url",
		Usage:    "Specify appchain master-rule url",
		Required: true,
	}
	appchainAdminFlag = cli.StringFlag{
		Name:     "admin",
		Usage:    "Specify appchain admin addr list, multiple addresses are separated by \",\". The current user is included by default.",
		Required: false,
	}
	appchainConsensusFlag = cli.StringFlag{
		Name:     "consensus",
		Usage:    "Specific appchain consensus type",
		Required: true,
	}
	governanceReasonFlag = cli.StringFlag{
		Name:     "reason",
		Usage:    "Specify governance reason",
		Required: false,
	}
	appchainRuleFlag = cli.StringFlag{
		Name:     "rule",
		Usage:    "Specify appchain rule addr",
		Required: false,
	}
	appchainRuleUrlFlag = cli.StringFlag{
		Name:     "rule-url",
		Usage:    "Specify appchain rule url",
		Required: false,
	}
)
