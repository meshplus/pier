module github.com/meshplus/pier

go 1.13

require (
	github.com/Rican7/retry v0.1.0
	github.com/btcsuite/btcd v0.20.1-beta
	github.com/cbergoon/merkletree v0.2.0
	github.com/fatih/color v1.9.0
	github.com/fsnotify/fsnotify v1.4.7
	github.com/gin-gonic/gin v1.6.3
	github.com/gobuffalo/logger v1.0.0
	github.com/gobuffalo/packd v1.0.0
	github.com/gobuffalo/packr v1.30.1
	github.com/gogo/protobuf v1.3.1
	github.com/golang/mock v1.4.3
	github.com/hashicorp/go-hclog v0.0.0-20180709165350-ff2cf002a8dd
	github.com/hashicorp/go-plugin v1.3.0
	github.com/ipfs/go-cid v0.0.7
	github.com/lestrrat-go/strftime v1.0.3 // indirect
	github.com/libp2p/go-libp2p-core v0.6.1
	github.com/meshplus/bitxhub-core v0.1.0-rc1.0.20201021153523-274a013bfd41
	github.com/meshplus/bitxhub-kit v1.1.2-0.20201203072410-8a0383a6870d
	github.com/meshplus/bitxhub-model v1.1.2-0.20201221070800-ca8184215353
	github.com/meshplus/go-bitxhub-client v1.0.0-rc4.0.20210112021415-f07419870758
	github.com/meshplus/go-lightp2p v0.0.0-20200817105923-6b3aee40fa54
	github.com/mitchellh/go-homedir v1.1.0
	github.com/multiformats/go-multiaddr v0.3.0
	github.com/sirupsen/logrus v1.6.0
	github.com/spf13/viper v1.6.1
	github.com/stretchr/testify v1.6.0
	github.com/tidwall/gjson v1.6.0
	github.com/urfave/cli v1.22.1
	go.uber.org/atomic v1.6.0
	golang.org/x/tools v0.0.0-20201009162240-fcf82128ed91 // indirect
	google.golang.org/grpc v1.33.1
)

replace github.com/libp2p/go-libp2p-core => github.com/libp2p/go-libp2p-core v0.5.6
