package main

import (
	"fmt"
	"github.com/meshplus/pier/internal/repo"
	"github.com/spf13/viper"
	"github.com/urfave/cli"
	"net"
	"path/filepath"
	"strings"
)

var networkCMD = cli.Command{
	Name:  "network",
	Usage: "Modify pier network configuration",
	Subcommands: []cli.Command{
		{
			Name:  "relay",
			Usage: "Modify pier relay mode network configuration",
			Flags: []cli.Flag{
				cli.StringSliceFlag{
					Name:     "addNodes",
					Usage:    "add bitxhub node address",
					Required: false,
				},
				cli.StringSliceFlag{
					Name:     "delNodes",
					Usage:    "delete bitxhub node address",
					Required: false,
				},
			},
			Action: configNetwork,
		},
		{
			Name:  "direct",
			Usage: "Modify pier direct mode network configuration",
			Flags: []cli.Flag{
				cli.StringSliceFlag{
					Name:     "addPier",
					Usage:    "add pier address and pid, split with \"#\" ",
					Required: false,
				},
				cli.StringSliceFlag{
					Name:     "delPier",
					Usage:    "delete pier address and pid, split with \"#\" ",
					Required: false,
				},
			},
			Action: configNetwork,
		},
		{
			Name:  "union",
			Usage: "Modify pier union mode network configuration",
			Flags: []cli.Flag{
				cli.StringSliceFlag{
					Name:     "addNodes",
					Usage:    "add bitxhub node address",
					Required: false,
				},
				cli.StringSliceFlag{
					Name:     "delNodes",
					Usage:    "delete bitxhub node address",
					Required: false,
				},
				cli.StringSliceFlag{
					Name:     "addPier",
					Usage:    "add pier address and pid, split with \"#\" ",
					Required: false,
				},
				cli.StringSliceFlag{
					Name:     "delPier",
					Usage:    "add pier address and pid, split with \"#\" ",
					Required: false,
				},
			},
			Action: configNetwork,
		},
	},
}

func configNetwork(ctx *cli.Context) error {
	repoRoot, err := repo.PathRootWithDefault(ctx.GlobalString("repo"))
	if err != nil {
		return err
	}

	pv, err := ReadFromConfigFile(filepath.Join(repoRoot, repo.ConfigName))
	if err != nil {
		return err
	}
	nv, err := ReadFromConfigFile(filepath.Join(repoRoot, repo.NetworkName))
	if err != nil {
		return err
	}

	mode := ctx.Command.Name
	switch mode {
	case "relay":
		adrrs, err := updateBxhAddrs(ctx, pv, mode)
		if err != nil {
			return err
		}
		pv.Set("mode.relay.addrs", adrrs)
		if err := pv.WriteConfig(); err != nil {
			return err
		}
	case "direct":
		oldConfig := &repo.NetworkConfig{}
		// fresh old networkConfig.
		if err := repo.ReadConfig(nv, filepath.Join(repoRoot, "network.toml"), "toml", oldConfig); err != nil {
			return err
		}
		err := updateNetworkAddrs(ctx, oldConfig, repoRoot, mode)
		if err != nil {
			return err
		}
	case "union":
		if len(ctx.StringSlice("addNodes")) != 0 || len(ctx.StringSlice("delNodes")) != 0 {
			adrrs, err := updateBxhAddrs(ctx, pv, mode)
			if err != nil {
				return err
			}
			pv.Set("mode.union.addrs", adrrs)
			if err := pv.WriteConfig(); err != nil {
				return err
			}
		} else {
			oldConfig := &repo.NetworkConfig{}
			// fresh old networkConfig.
			if err := repo.ReadConfig(nv, filepath.Join(repoRoot, "network.toml"), "toml", oldConfig); err != nil {
				return err
			}
			err = updateNetworkAddrs(ctx, oldConfig, repoRoot, mode)
			if err != nil {
				return err
			}
		}

	}
	return nil

}

func updateBxhAddrs(ctx *cli.Context, vpr *viper.Viper, mode string) ([]string, error) {
	var (
		oldAddrs    []string
		addBxhAddrs = ctx.StringSlice("addNodes")
		delBxhAddrs = ctx.StringSlice("delNodes")
	)

	switch mode {
	case "relay":
		oldAddrs = vpr.GetStringSlice("mode.relay.addrs")
	case "uninon":
		oldAddrs = vpr.GetStringSlice("mode.union.addrs")
	default:
		err := fmt.Errorf("unsupport mode type: %s", mode)
		return nil, err
	}

	tmpAddrs := make(map[string]bool)
	for _, addr := range oldAddrs {
		tmpAddrs[addr] = true
	}

	for _, newAddrs := range addBxhAddrs {
		for _, newAddr := range strings.Split(newAddrs, ",") {
			// if addr repeat, key is unique.
			tmpAddrs[newAddr] = true
		}

	}

	for _, delAddrs := range delBxhAddrs {
		for _, delAddr := range strings.Split(delAddrs, ",") {
			// if addr does not exit, return err.
			if ok := tmpAddrs[delAddr]; !ok {
				return nil, fmt.Errorf("addr does not exist")
			}
			delete(tmpAddrs, delAddr)
		}

	}

	var newAddrs []string
	for k, _ := range tmpAddrs {
		if k == "" {
			continue
		}
		newAddrs = append(newAddrs, k)
	}
	return newAddrs, nil
}

func updateNetworkAddrs(ctx *cli.Context, oldConfig *repo.NetworkConfig, repoRoot string, mode string) error {
	if mode != "direct" && mode != "union" {
		return fmt.Errorf("unsupport mode type: %s", mode)
	}

	var (
		newPiers []*repo.NetworkPiers
		//record pier.
		pierMap  = make(map[string]*repo.NetworkPiers)
		addPiers = ctx.StringSlice("addPier")
		delPiers = ctx.StringSlice("delPier")
	)

	// fresh old networkConfig.
	if oldConfig != nil {
		for _, oldPier := range oldConfig.Piers {
			pierMap[oldPier.Pid] = oldPier
		}
	}

	for _, str := range addPiers {
		// multiStr looks like the format of QMfewfxxxxxxx#ip1:port1,ip2:port2....
		multiStr := strings.Split(str, "#")
		var hosts []string
		// the input must including one "#"
		if len(multiStr) != 2 {
			return fmt.Errorf("illegal pier input:%s, correct input looks like: [ip]:[port]#[Pid]", str)
		}
		// addr looks like the format of ip1:port1,ip2:port2....
		for _, addr := range strings.Split(multiStr[0], ",") {
			host := strings.Split(addr, ":")
			if len(host) != 2 {
				return fmt.Errorf(
					"illegal host input:%s, correct input looks like: [ip1]:[port1],[ip2]:[port2]", host)
			}
			ip := net.ParseIP(host[0])
			if ip == nil {
				return fmt.Errorf("illegal type of ipv4: %s", host[0])
			}
			port := host[1]
			peer := fmt.Sprintf("/ip4/%s/tcp/%s/p2p", ip, port)
			hosts = append(hosts, peer)
		}
		// input new Pier to pierMap.
		if _, ok := pierMap[multiStr[1]]; !ok {
			pierMap[multiStr[1]] = &repo.NetworkPiers{
				Pid:   multiStr[1],
				Hosts: hosts,
			}
		} else {
			return fmt.Errorf("reapt pid")
		}
	}

	for _, str := range delPiers {
		mutiStr := strings.Split(str, "#")
		if len(mutiStr) != 2 {
			return fmt.Errorf("illegal pier input:%s, correct input looks like: [ip]:[port]#[Pid]", str)
		}
		// remove pier form pierMap. if does not exist, return err.
		if _, ok := pierMap[mutiStr[1]]; ok {
			delete(pierMap, mutiStr[1])
		} else {
			return fmt.Errorf("%s does not exit", str)
		}
	}

	for _, pier := range pierMap {
		newPiers = append(newPiers, pier)
	}

	changeConfig := &repo.NetworkConfig{
		Piers: newPiers,
	}
	//write config into network.toml
	if err := repo.WriteNetworkConfig("", repoRoot, changeConfig); err != nil {
		return err
	}
	return nil
}
