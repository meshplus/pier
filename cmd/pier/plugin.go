package main

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/cavaliercoder/grab"
	"github.com/gobuffalo/packd"
	packr2 "github.com/gobuffalo/packr/v2"
	"github.com/meshplus/bitxhub-kit/fileutil"
	"github.com/meshplus/bitxhub-kit/types"
	"github.com/meshplus/pier/internal/repo"
	"github.com/spf13/viper"
	"github.com/urfave/cli"
)

// TODO: set url
var binaryUrl = map[string]string{
	"fabric":   "https://github.com/meshplus/pier-client-fabric/releases/download/v1.11.1/fabric-client-v1.11.1-%s",
	"ethereum": "https://github.com/meshplus/pier-client-ethereum/releases/download/v1.14.0/eth-client-v1.14.0-%s",
	//"flato":    "https://raw.githubusercontent.com/meshplus/pier-client-ethereum/release-1.11/config/data_swapper.abi",
	//"bcos":     "https://raw.githubusercontent.com/meshplus/pier-client-ethereum/release-1.11/config/data_swapper.abi",
}

var pluginCMD = cli.Command{
	Name:  "plugin",
	Usage: "Initialize pier plugins configuration",
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:     "redownload",
			Usage:    "Re-download plugin or not",
			Required: false,
		},
	},
	Subcommands: []cli.Command{
		{
			Name:  "fabric",
			Usage: "Initialize pier and fabric plugin configuration",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:     "crypto-config",
					Usage:    "Specify the path to crypto-config directory",
					Required: true,
				},
				cli.StringFlag{
					Name:     "config",
					Usage:    "Specify the path to fabric config.yaml",
					Required: true,
				},
				//todo: remove it
				cli.StringFlag{
					Name:     "event-filter",
					Usage:    "Specify the event filter on fabric chaincode",
					Required: false,
				},
				cli.StringFlag{
					Name:     "username",
					Usage:    "Specify the username to invoke fabric chaincode",
					Required: false,
				},
				cli.StringFlag{
					Name:     "ccid",
					Usage:    "Specify chaincode id to invoke",
					Required: false,
				},
				cli.StringFlag{
					Name:     "channel-id",
					Usage:    "Specify channel id",
					Required: false,
				},
				cli.StringFlag{
					Name:     "org",
					Usage:    "Specify the organization",
					Required: false,
				},
				//cli.StringFlag{
				//	Name:     "timeout-height",
				//	Usage:    "Specify the ibtp timeout block height",
				//	Required: false,
				//},
			},
			Action: configPlugin,
		},
		{
			Name:  "ethereum",
			Usage: "Initialize pier and ethereum plugin configuration",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:     "addr",
					Usage:    "Specify ethereum websocket address",
					Required: false,
				},
				cli.StringFlag{
					Name:     "broker",
					Usage:    "Specify ethereum broker contract address",
					Required: true,
				},
				cli.StringFlag{
					Name:     "key",
					Usage:    "Specify the ethereum key to sign the transaction",
					Required: false,
				},
				cli.StringFlag{
					Name:     "password",
					Usage:    "Specify the password of the key",
					Required: false,
				},
				cli.Uint64Flag{
					Name:     "min-confirm",
					Usage:    "Specify minimum blocks to confirm the transaction",
					Required: false,
				},
				cli.StringFlag{
					Name:     "transfer",
					Usage:    "Specify the transfer contract address",
					Required: false,
				},
				cli.StringFlag{
					Name:     "data-swapper",
					Usage:    "Specify the data swapper contract address",
					Required: false,
				},
				cli.StringFlag{
					Name:     "timeout-height",
					Usage:    "Specify the ibtp timeout block height",
					Required: false,
				},
			},
			Action: configPlugin,
		},
		//{
		//	Name:  "flato",
		//	Usage: "Initialize pier and flato plugin configuration",
		//	Flags: []cli.Flag{
		//		cli.StringFlag{
		//			Name:     "broker",
		//			Usage:    "Specify flato broker contract address",
		//			Required: true,
		//		},
		//		cli.StringFlag{
		//			Name:     "queue-name",
		//			Usage:    "Specify flato MQ queue name bound to exhcange",
		//			Required: false,
		//		},
		//		cli.StringFlag{
		//			Name:     "username",
		//			Usage:    "Specify the username to visit MQ",
		//			Required: false,
		//			Value:    "guest",
		//		},
		//		cli.StringFlag{
		//			Name:     "password",
		//			Usage:    "Specify the password to visit MQ",
		//			Required: false,
		//			Value:    "guest",
		//		},
		//		cli.StringFlag{
		//			Name:     "exchange",
		//			Usage:    "Specify flato MQ exchange registered to broker contract",
		//			Required: true,
		//		},
		//		cli.StringFlag{
		//			Name:     "contract-type",
		//			Usage:    "Specify flato MQ exchange registered to broker contract",
		//			Required: false,
		//		},
		//		cli.StringSliceFlag{
		//			Name:     "validators",
		//			Usage:    "Specify flato validators",
		//			Required: false,
		//		},
		//		cli.StringFlag{
		//			Name:     "transfer",
		//			Usage:    "Specify the transfer contract address",
		//			Required: false,
		//		},
		//		cli.StringFlag{
		//			Name:     "data-swapper",
		//			Usage:    "Specify the data swapper contract address",
		//			Required: false,
		//		},
		//		cli.StringFlag{
		//			Name:     "license-key",
		//			Usage:    "Specify license key",
		//			Required: false,
		//		},
		//		cli.StringFlag{
		//			Name:     "verifier",
		//			Usage:    "Specify license verifier",
		//			Required: false,
		//		},
		//		cli.StringFlag{
		//			Name:     "license",
		//			Usage:    "Specify the path to license file",
		//			Required: false,
		//		},
		//		cli.StringSliceFlag{
		//			Name:     "nodes",
		//			Usage:    "Specify the json-rpc address of flato nodes (ip:port or ip), default port is 8081",
		//			Required: false,
		//		},
		//	},
		//	Action: configPlugin,
		//},
		//{
		//	Name:  "bcos",
		//	Usage: "Initialize pier and bcos plugin configuration",
		//	Flags: []cli.Flag{
		//		cli.StringFlag{
		//			Name:     "broker",
		//			Usage:    "Specify flato broker contract address",
		//			Required: true,
		//		},
		//		cli.StringFlag{
		//			Name:     "transfer",
		//			Usage:    "Specify the transfer contract address",
		//			Required: false,
		//		},
		//		cli.StringFlag{
		//			Name:     "data-swapper",
		//			Usage:    "Specify the data swapper contract address",
		//			Required: false,
		//		},
		//		cli.StringSliceFlag{
		//			Name:     "nodes",
		//			Usage:    "Specify the address of bcos nodes (ip:port or ip), default port is 20200",
		//			Required: false,
		//		},
		//	},
		//	Action: configPlugin,
		//},
	},
}

func configPlugin(ctx *cli.Context) error {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		return fmt.Errorf("current os %s is not supported, please use darwin or linux", runtime.GOOS)
	}

	repoRoot, err := repo.PathRootWithDefault(ctx.GlobalString("repo"))
	if err != nil {
		return err
	}

	chainType := ctx.Command.Name
	configDir := filepath.Join(repoRoot, chainType)

	if fileutil.Exist(configDir) {
		fmt.Printf("%s plugin configuration file already exists\n", chainType)
		fmt.Println("reinitializing would overwrite your configuration, Y/N?")
		input := bufio.NewScanner(os.Stdin)
		input.Scan()
		if input.Text() == "Y" || input.Text() == "y" {
			if err := os.RemoveAll(configDir); err != nil {
				return err
			}
		} else {
			return nil
		}
	}

	if err := os.Mkdir(configDir, 0755); err != nil {
		return err
	}

	plugin := filepath.Join(repoRoot, "plugins", chainType)
	if err := downloadFile(plugin, getUrl(chainType), ctx.GlobalBool("redownload")); err != nil {
		return err
	}

	if err := updateAppchainType(repoRoot, chainType); err != nil {
		return err
	}

	cmd := exec.Command(plugin, "init", "--target", configDir)
	if err := cmd.Run(); err != nil {
		return err
	}

	if err := updateAppchainConfig(ctx, configDir, chainType); err != nil {
		return err
	}
	return err
}

func updateAppchainConfig(ctx *cli.Context, configDir, chainType string) error {
	configFile := filepath.Join(configDir, fmt.Sprintf("%s.toml", chainType))
	v := viper.New()
	v.SetConfigFile(configFile)
	v.SetConfigType("toml")
	if err := v.ReadInConfig(); err != nil {
		return err
	}
	defer func() {
		err := v.WriteConfig()
		if err != nil {
			fmt.Println(err)
		}
	}()

	switch chainType {
	case "fabric":
		setNonemptyString(v, "username", ctx.String("username"))
		setNonemptyString(v, "ccid", ctx.String("ccid"))
		setNonemptyString(v, "channel_id", ctx.String("channel-id"))
		setNonemptyString(v, "org", ctx.String("org"))
		//setNonemptyString(v, "timeout_height", ctx.String("timeout-height"))
		if err := v.WriteConfig(); err != nil {
			return err
		}

		if cryptoConfig := ctx.String("crypto-config"); cryptoConfig != "" {
			if err := copyDir(cryptoConfig, filepath.Join(configDir, "crypto-config")); err != nil {
				return err
			}
		}

		if config := ctx.String("config"); config != "" {
			if err := copyFile(filepath.Join(config, "config.yaml"), filepath.Join(configDir, "config.yaml")); err != nil {
				return err
			}
		}
	case "ethereum":
		setNonemptyString(v, "ether.addr", ctx.String("addr"))
		setNonemptyString(v, "ether.contract_address", ctx.String("broker"))
		setNonemptyString(v, "ether.min_confirm", ctx.String("min-confirm"))
		setNonemptyString(v, "ether.timeout_height", ctx.String("timeout-height"))

		if broker := ctx.String("broker"); broker != "" {
			addr := types.NewAddressByStr(broker).String()
			if !strings.EqualFold(addr, broker) {
				return fmt.Errorf("invalid contract address")
			}
			v.Set("ether.contract_address", addr)
		}

		if err := setABI(ctx, "transfer", v, true); err != nil {
			return err
		}

		if err := setABI(ctx, "data-swapper", v, true); err != nil {
			return err
		}

		if err := v.WriteConfig(); err != nil {
			return err
		}

		if key := ctx.String("key"); key != "" {
			if err := copyFile(key, filepath.Join(configDir, "account.key")); err != nil {
				return err
			}
		}

		if password := ctx.String("password"); password != "" {
			if err := ioutil.WriteFile(filepath.Join(configDir, "password"), []byte(password), 0644); err != nil {
				return err
			}
		}
	case "flato":
		setNonemptyString(v, "flato.contract_address", ctx.String("broker"))
		setNonemptyString(v, "flato.queue_name", ctx.String("queue_name"))
		setNonemptyString(v, "flato.user_name", ctx.String("user_name"))
		setNonemptyString(v, "flato.password", ctx.String("password"))
		setNonemptyString(v, "flato.exchange", ctx.String("exchange"))
		setNonemptyString(v, "flato.contract_type", ctx.String("contract_type"))
		setNonemptyString(v, "flato.validators", ctx.String("validators"))

		if err := setABI(ctx, "transfer", v, false); err != nil {
			return err
		}

		if err := setABI(ctx, "data-swapper", v, false); err != nil {
			return err
		}

		setNonemptyString(v, "license.key", ctx.String("license-key"))
		setNonemptyString(v, "license.verifier", ctx.String("verifier"))

		if license := ctx.String("license"); license != "" {
			if err := copyFile(license, filepath.Join(configDir, "LICENSE")); err != nil {
				return err
			}
		}

		if err := setHpcConfig(ctx, configDir); err != nil {
			return err
		}
	case "bcos":
		setNonemptyString(v, "bcos.contract_address", ctx.String("broker"))

		if err := setABI(ctx, "transfer", v, false); err != nil {
			return err
		}

		if err := setABI(ctx, "data-swapper", v, false); err != nil {
			return err
		}

		if err := setBcosConfig(ctx, configDir); err != nil {
			return err
		}
	}

	return nil
}

func updateAppchainType(repoRoot, chainType string) error {
	configFile := filepath.Join(repoRoot, repo.ConfigName)
	return updateTomlConfig(configFile, func(v *viper.Viper) {
		v.Set("appchain.plugin", chainType)
		v.Set("appchain.config", chainType)
	})
}

func copyDir(srcDir, dstDir string) error {
	box := packr2.New("box", srcDir)
	return box.Walk(func(s string, file packd.File) error {
		p := filepath.Join(dstDir, s)
		dir := filepath.Dir(p)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			err := os.MkdirAll(dir, 0755)
			if err != nil {
				return err
			}
		}
		return ioutil.WriteFile(p, []byte(file.String()), 0644)
	})
}

func copyFile(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()

	_, err = io.Copy(destination, source)
	return err
}

func downloadFile(dst, url string, overwrite bool) error {
	if !overwrite && fileutil.Exist(dst) {
		return nil
	}

	fmt.Println("Start downloading", url)
	if _, err := grab.Get(dst, url); err != nil {
		return err
	}
	fmt.Println("Finish downloading", url)

	return os.Chmod(dst, 0755)
}

func setABI(ctx *cli.Context, bizName string, v *viper.Viper, checksum bool) error {
	abiName := fmt.Sprintf("%s.abi", strings.ReplaceAll(bizName, "-", "_"))
	bizAddr := ctx.String(bizName)
	if bizAddr != "" {
		if checksum {
			addr := types.NewAddressByStr(bizAddr).String()
			if !strings.EqualFold(addr, bizAddr) {
				return fmt.Errorf("invalid biz address: %s", bizAddr)
			}
			bizAddr = addr
		}
		v.Set(fmt.Sprintf("contract_abi.%s", bizAddr), abiName)
	}

	return nil
}

func setNonemptyString(v *viper.Viper, key, val string) {
	if val != "" {
		v.Set(key, val)
	}
}

func setHpcConfig(ctx *cli.Context, configDir string) error {
	var (
		addrs []string
		ports []string
	)

	nodes := ctx.StringSlice("nodes")
	if len(nodes) == 0 {
		return nil
	}
	for _, node := range nodes {
		splits := strings.Split(node, ":")
		if len(splits) > 2 {
			return fmt.Errorf("invalid node address: %s", node)
		}
		addrs = append(addrs, splits[0])
		if len(splits) == 2 {
			ports = append(ports, splits[1])
		} else {
			ports = append(ports, "8081")
		}
	}

	configFile := filepath.Join(configDir, "hpc.toml")
	return updateTomlConfig(configFile, func(v *viper.Viper) {
		v.Set("jsonRPC.nodes", addrs)
		v.Set("jsonRPC.ports", ports)
	})
}

func setBcosConfig(ctx *cli.Context, configDir string) error {
	nodes := ctx.StringSlice("nodes")
	if len(nodes) == 0 {
		return nil
	}
	for _, node := range nodes {
		if len(strings.Split(node, ":")) > 2 {
			return fmt.Errorf("invalid node address: %s", node)
		}
	}

	configFile := filepath.Join(configDir, "config.toml")
	return updateTomlConfig(configFile, func(v *viper.Viper) {
		v.Set("Network.Connection.NodeURL", nodes[0])
		//v.Set("jsonRPC.nodes", addrs)
		//v.Set("jsonRPC.ports", ports)
	})
}

func updateTomlConfig(configFile string, handler func(v *viper.Viper)) error {
	v := viper.New()
	v.SetConfigFile(configFile)
	v.SetConfigType("toml")
	if err := v.ReadInConfig(); err != nil {
		return err
	}

	handler(v)

	return v.WriteConfig()
}

func getUrl(chainType string) string {
	return fmt.Sprintf(binaryUrl[chainType], runtime.GOOS)
}
