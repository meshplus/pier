module github.com/meshplus/pier

go 1.13

require (
	github.com/Rican7/retry v0.1.0
	github.com/bitxhub/crypto-gm v0.0.0-00010101000000-000000000000
	github.com/bitxhub/offchain-transmission v0.0.0-00010101000000-000000000000
	github.com/bitxhub/pier-ha v0.0.0-20230714112049-004e7dc7639d
	github.com/btcsuite/btcd v0.21.0-beta
	github.com/cbergoon/merkletree v0.2.0
	github.com/fatih/color v1.9.0
	github.com/fsnotify/fsnotify v1.4.9
	github.com/gin-gonic/gin v1.6.3
	github.com/gobuffalo/packd v1.0.0
	github.com/gobuffalo/packr v1.30.1
	github.com/gogo/protobuf v1.3.2
	github.com/golang/mock v1.5.0
	github.com/hashicorp/go-hclog v0.0.0-20180709165350-ff2cf002a8dd
	github.com/hashicorp/go-plugin v1.3.0
	github.com/ipfs/go-cid v0.0.7
	github.com/libp2p/go-libp2p-core v0.6.1
	github.com/meshplus/bitxhub-core v1.3.1-0.20220511024304-f7458609c30a
	github.com/meshplus/bitxhub-kit v1.2.1-0.20220325052414-bc17176c509d
	github.com/meshplus/bitxhub-model v1.2.1-0.20220616031805-96a66092bc97
	github.com/meshplus/go-bitxhub-client v1.0.0-rc4.0.20210416022059-22729ce4c0f2
	github.com/meshplus/go-lightp2p v0.0.0-20200817105923-6b3aee40fa54
	github.com/mitchellh/go-homedir v1.1.0
	github.com/multiformats/go-multiaddr v0.3.0
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/viper v1.7.1
	github.com/stretchr/testify v1.7.0
	github.com/tidwall/gjson v1.6.8
	github.com/urfave/cli v1.22.1
	github.com/wonderivan/logger v1.0.0
	go.uber.org/atomic v1.7.0
	google.golang.org/grpc v1.33.2
)

replace github.com/meshplus/bitxhub-model => github.com/meshplus/bitxhub-model v1.1.2-0.20230717034022-af48720dde35

replace github.com/bitxhub/pier-ha => git.hyperchain.cn/bitxhub/pier-ha v0.0.0-20230714112049-004e7dc7639d

replace github.com/libp2p/go-libp2p-core => github.com/libp2p/go-libp2p-core v0.5.6

replace google.golang.org/grpc => google.golang.org/grpc v1.33.1

replace golang.org/x/net => golang.org/x/net v0.0.0-20201021035429-f5854403a974

replace github.com/bitxhub/crypto-gm => git.hyperchain.cn/bitxhub/bxh-crypto-gm v0.0.0-20210825015341-e035b646648d

replace github.com/bitxhub/offchain-transmission => git.hyperchain.cn/bitxhub/offchain-transmission v0.0.0-20220616032739-b2eba23f65b7

replace github.com/ultramesh/crypto-gm => git.hyperchain.cn/ultramesh/crypto-gm v0.2.14
