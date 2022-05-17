module github.com/meshplus/pier

go 1.13

require (
	github.com/Rican7/retry v0.1.0
	github.com/btcsuite/btcd v0.21.0-beta
	github.com/cavaliercoder/grab v2.0.0+incompatible
	github.com/cbergoon/merkletree v0.2.0
	github.com/ethereum/go-ethereum v1.10.7
	github.com/fatih/color v1.9.0
	github.com/fsnotify/fsnotify v1.4.9
	github.com/gin-gonic/gin v1.6.3
	github.com/gobuffalo/packd v1.0.0
	github.com/gobuffalo/packr v1.30.1
	github.com/golang/mock v1.5.0
	github.com/google/btree v1.0.0
	github.com/hashicorp/go-hclog v0.0.0-20180709165350-ff2cf002a8dd
	github.com/hashicorp/go-plugin v1.3.0
	github.com/ipfs/go-cid v0.0.7
	github.com/libp2p/go-libp2p v0.9.2
	github.com/libp2p/go-libp2p-core v0.6.1
	github.com/meshplus/bitxhub-core v1.3.1-0.20220129100150-aa3fd98079e4
	github.com/meshplus/bitxhub-kit v1.2.1-0.20210902085548-07f4fa85bfc9
	github.com/meshplus/bitxhub-model v1.2.1-0.20220425093801-4cc50cc6bc61
	github.com/meshplus/bitxid v0.0.0-20210412025850-e0eaf0f9063a
	github.com/meshplus/go-bitxhub-client v1.3.1-0.20210701063659-a0836fbc1c78
	github.com/meshplus/go-lightp2p v0.0.0-20200817105923-6b3aee40fa54
	github.com/mitchellh/go-homedir v1.1.0
	github.com/multiformats/go-multiaddr v0.3.0
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/viper v1.7.1
	github.com/stretchr/testify v1.7.0
	github.com/tidwall/gjson v1.6.8
	github.com/urfave/cli v1.22.1
	github.com/wasmerio/wasmer-go v1.0.4 // indirect
	github.com/wonderivan/logger v1.0.0
	go.uber.org/atomic v1.7.0
	golang.org/x/sys v0.0.0-20220503163025-988cb79eb6c6 // indirect
	google.golang.org/grpc v1.33.2
)

replace github.com/libp2p/go-libp2p-core => github.com/libp2p/go-libp2p-core v0.5.6

replace golang.org/x/net => golang.org/x/net v0.0.0-20210405180319-a5a99cb37ef4

//replace github.com/meshplus/go-lightp2p => git.hyperchain.cn/dmlab/go-lightp2p v0.0.0-20220513125259-25519f9c6fa3
replace github.com/meshplus/go-lightp2p => ../../dmlab/go-lightp2p

replace git.hyperchain.cn/dmlab/go-common-utils => git.hyperchain.cn/dmlab/go-common-utils.git v0.0.0-20200323065116-07edae98cb7a

replace github.com/hyperchain/gosdk => git.hyperchain.cn/hyperchain/gosdk.git v1.2.16

replace github.com/ultramesh/crypto-standard => git.hyperchain.cn/ultramesh/crypto-standard.git v0.1.0

replace github.com/ultramesh/crypto => git.hyperchain.cn/ultramesh/crypto.git v0.1.0
