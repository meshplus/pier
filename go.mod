module github.com/meshplus/pier

go 1.13

require (
	github.com/Rican7/retry v0.1.0
	github.com/btcsuite/btcd v0.21.0-beta
	github.com/cbergoon/merkletree v0.2.0
	github.com/ethereum/go-ethereum v1.10.4
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
	github.com/lestrrat-go/strftime v1.0.3 // indirect
	github.com/libp2p/go-libp2p-core v0.6.1
	github.com/meshplus/bitxhub-core v1.3.1-0.20210611011450-ca11d623d8cc
	github.com/meshplus/bitxhub-kit v1.2.1-0.20210616114532-4849447f09e1
	github.com/meshplus/bitxhub-model v1.2.1-0.20210624072142-369d0f708c9b
	github.com/meshplus/bitxid v0.0.0-20210412025850-e0eaf0f9063a
	github.com/meshplus/go-bitxhub-client v1.3.0
	github.com/meshplus/go-lightp2p v0.0.0-20200817105923-6b3aee40fa54
	github.com/mitchellh/go-homedir v1.1.0
	github.com/multiformats/go-multiaddr v0.3.0
	github.com/sirupsen/logrus v1.6.0
	github.com/spf13/viper v1.7.0
	github.com/stretchr/testify v1.7.0
	github.com/tidwall/gjson v1.6.8
	github.com/urfave/cli v1.22.1
	github.com/wonderivan/logger v1.0.0
	go.uber.org/atomic v1.6.0
	google.golang.org/grpc v1.33.1
)

replace github.com/libp2p/go-libp2p-core => github.com/libp2p/go-libp2p-core v0.5.6

replace github.com/meshplus/bitxhub-model => /Users/windiyi/GolandProjects/bitxhub-model

replace github.com/meshplus/go-bitxhub-client => /Users/windiyi/GolandProjects/go-bitxhub-client
