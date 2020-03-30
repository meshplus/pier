module github.com/meshplus/pier

go 1.13

require (
	github.com/Rican7/retry v0.1.0
	github.com/fatih/color v1.9.0
	github.com/fsnotify/fsnotify v1.4.7
	github.com/gobuffalo/envy v1.8.1 // indirect
	github.com/gobuffalo/packd v0.3.0
	github.com/gobuffalo/packr v1.30.1
	github.com/golang/mock v1.3.1
	github.com/meshplus/bitxhub-kit v1.0.0-rc1
	github.com/meshplus/bitxhub-model v1.0.0-rc1
	github.com/meshplus/go-bitxhub-client v1.0.0-rc1
	github.com/mitchellh/go-homedir v1.1.0
	github.com/rogpeppe/go-internal v1.5.1 // indirect
	github.com/sirupsen/logrus v1.4.2
	github.com/spf13/viper v1.4.0
	github.com/stretchr/testify v1.4.0
	github.com/syndtr/goleveldb v1.0.1-0.20190923125748-758128399b1d
	github.com/tidwall/gjson v1.6.0
	github.com/urfave/cli v1.22.1
	golang.org/x/sys v0.0.0-20191228213918-04cbcbbfeed8 // indirect
	golang.org/x/text v0.3.2 // indirect
)

replace golang.org/x/net => golang.org/x/net v0.0.0-20200202094626-16171245cfb2

replace github.com/mholt/archiver => github.com/mholt/archiver v0.0.0-20180417220235-e4ef56d48eb0

replace github.com/ugorji/go v1.1.4 => github.com/ugorji/go/codec v0.0.0-20190204201341-e444a5086c43
