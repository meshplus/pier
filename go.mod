module github.com/meshplus/pier

go 1.13

require (
	github.com/Rican7/retry v0.1.0
	github.com/btcsuite/btcd v0.21.0-beta
	github.com/bytecodealliance/wasmtime-go v0.37.0 // indirect
	github.com/cavaliercoder/grab v2.0.0+incompatible
	github.com/cbergoon/merkletree v0.2.0
	github.com/fatih/color v1.9.0
	github.com/fsnotify/fsnotify v1.4.9
	github.com/gobuffalo/packd v1.0.1
	github.com/gobuffalo/packr/v2 v2.8.3
	github.com/golang/mock v1.6.0
	github.com/google/btree v1.0.0
	github.com/hashicorp/go-hclog v0.0.0-20180709165350-ff2cf002a8dd
	github.com/hashicorp/go-plugin v1.3.0
	github.com/ipfs/go-cid v0.0.7
	github.com/libp2p/go-libp2p-core v0.6.1
	github.com/meshplus/bitxhub-core v1.3.1-0.20220511024304-f7458609c30a
	github.com/meshplus/bitxhub-kit v1.2.1-0.20220412092457-5836414df781
	github.com/meshplus/bitxhub-model v1.2.1-0.20220616031805-96a66092bc97
	github.com/meshplus/go-bitxhub-client v1.4.1-0.20220412093230-11ca79f069fc
	github.com/meshplus/go-lightp2p v0.0.0-20200817105923-6b3aee40fa54
	github.com/mitchellh/go-homedir v1.1.0
	github.com/multiformats/go-multiaddr v0.3.0
	github.com/pelletier/go-toml v1.9.3
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/viper v1.8.1
	github.com/stretchr/testify v1.8.0
	github.com/tidwall/gjson v1.6.8
	github.com/urfave/cli v1.22.1
	github.com/wonderivan/logger v1.0.0
	go.uber.org/atomic v1.7.0
	google.golang.org/grpc v1.38.0
)

replace github.com/libp2p/go-libp2p-core => github.com/libp2p/go-libp2p-core v0.5.6

replace github.com/hyperledger/fabric => github.com/hyperledger/fabric v2.0.1+incompatible

replace golang.org/x/net => golang.org/x/net v0.0.0-20210405180319-a5a99cb37ef4

replace github.com/binance-chain/tss-lib => github.com/dawn-to-dusk/tss-lib v1.3.3-0.20220330081758-f404e10a1268
