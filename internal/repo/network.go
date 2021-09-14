package repo

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	crypto2 "github.com/libp2p/go-libp2p-core/crypto"
	peer2 "github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/pelletier/go-toml"
	"github.com/spf13/viper"
)

type NetworkConfig struct {
	LocalAddr string          `toml:"local_addr, omitempty" json:"local_addr"`
	Piers     []*NetworkPiers `toml:"piers" json:"piers"`
}

type NetworkPiers struct {
	Pid   string   `toml:"pid" json:"pid"`
	Hosts []string `toml:"hosts" json:"hosts"`
}

func LoadNetworkConfig(repoRoot string, privateKey crypto2.PrivKey) (*NetworkConfig, error) {
	var (
		v              = viper.New()
		networkConfig  = &NetworkConfig{}
		checkReaptAddr = make(map[string]string)
	)

	id, err := peer2.IDFromPrivateKey(privateKey)
	if err != nil {
		return nil, err
	}

	if err := ReadConfig(v, filepath.Join(repoRoot, "network.toml"), "toml", networkConfig); err != nil {
		return nil, err
	}

	for _, peer := range networkConfig.Piers {
		addr, err := ma.NewMultiaddr(fmt.Sprintf("%s%s", peer.Hosts[0], peer.Pid))
		if err != nil {
			return nil, fmt.Errorf("new multiaddr: %w", err)
		}
		if len(peer.Hosts) == 0 {
			return nil, fmt.Errorf("no hosts found by pier:%s", peer.Pid)
		}

		// remember localAddr
		if strings.Compare(peer.Pid, id.String()) == 0 {
			networkConfig.LocalAddr = peer.Hosts[0]
			networkConfig.LocalAddr = strings.Replace(
				networkConfig.LocalAddr, ma.Split(addr)[0].String(), "/ip4/0.0.0.0", -1)
		}

		// ensure the address is legitimate
		if _, ok := checkReaptAddr[peer.Hosts[0]]; !ok {
			checkReaptAddr[peer.Hosts[0]] = peer.Pid
		} else {
			return nil, fmt.Errorf("reapt address")
		}
	}

	if networkConfig.LocalAddr == "" {
		return nil, fmt.Errorf("lack of local address")
	}

	idx := strings.LastIndex(networkConfig.LocalAddr, "/p2p/")
	if idx == -1 {
		return nil, fmt.Errorf("pid is not existed in bootstrap")
	}

	networkConfig.LocalAddr = networkConfig.LocalAddr[:idx]

	return networkConfig, nil
}

func ReadConfig(v *viper.Viper, path, configType string, config interface{}) error {
	v.SetConfigFile(path)
	v.SetConfigType(configType)
	if err := v.ReadInConfig(); err != nil {
		return err
	}

	if err := v.Unmarshal(config); err != nil {
		return err
	}

	return nil
}

// GetNetworkPeers gets all peers from network config
func GetNetworkPeers(networkConfig *NetworkConfig) (map[string]*peer2.AddrInfo, error) {
	peers := make(map[string]*peer2.AddrInfo)
	for _, node := range networkConfig.Piers {
		if len(node.Hosts) == 0 {
			return nil, fmt.Errorf("no hosts found by node:%s", node.Pid)
		}
		multiaddr, err := ma.NewMultiaddr(fmt.Sprintf("%s%s", node.Hosts[0], node.Pid))
		if err != nil {
			return nil, fmt.Errorf("new Multiaddr error:%w", err)
		}
		addrInfo, err := peer2.AddrInfoFromP2pAddr(multiaddr)
		if err != nil {
			return nil, err
		}
		for i := 1; i < len(node.Hosts); i++ {
			multiaddr, err := ma.NewMultiaddr(fmt.Sprintf("%s%s", node.Hosts[i], node.Pid))
			if err != nil {
				return nil, fmt.Errorf("new Multiaddr error:%w", err)
			}
			addrInfo.Addrs = append(addrInfo.Addrs, multiaddr)
		}

		peers[node.Pid] = addrInfo
	}
	return peers, nil
}

func WriteNetworkConfig(originRoot string, repoRoot string, changeConfig *NetworkConfig) error {
	if originRoot != "" {
		fileData, err := ioutil.ReadFile(filepath.Join(originRoot, "network.toml"))
		if err != nil {
			fmt.Printf("err:  %s", err)
		}
		err = ioutil.WriteFile(filepath.Join(repoRoot, "network.toml"), fileData, 0644)
	}

	networkConfig := &NetworkConfig{}
	v := viper.New()
	v.SetConfigFile(filepath.Join(repoRoot, "network.toml"))
	v.SetConfigType("toml")
	if err := v.ReadInConfig(); err != nil {
		return err
	}
	if err := v.Unmarshal(networkConfig); err != nil {
		return err
	}

	networkConfig.Piers = changeConfig.Piers
	data, err := toml.Marshal(*networkConfig)
	if err != nil {
		return err
	}
	err = v.ReadConfig(bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	return v.WriteConfig()
}
