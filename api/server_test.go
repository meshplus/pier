package api

import (
	"encoding/json"
	"testing"

	"github.com/meshplus/bitxhub-kit/crypto"
	"github.com/meshplus/bitxhub-kit/crypto/asym"
	"github.com/meshplus/bitxhub-kit/hexutil"
	"github.com/meshplus/bitxhub-kit/types"
	"github.com/stretchr/testify/require"
)

const citaHeader = "{\"timestamp\":1621560062310,\"prevHash\":\"0x83455981490de6ed30be8c30617501029db32e8170470141eb2ceaa8a93d1b1e\",\"number\":\"0xb\",\"stateRoot\":\"0x6a614f6ca3938ba583de1d8c6c917d1786cb97867686d814ee612e6282b422e8\",\"transactionsRoot\":\"0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421\",\"receiptsRoot\":\"0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421\",\"quotaUsed\":\"0x0\",\"proof\":{\"tendermint\":null,\"Bft\":{\"proposal\":\"0xe9774e25a5546cd39f3ea021936436c61de93b1bbf87786f09759fd018f10bf8\",\"height\":10,\"round\":0,\"commits\":{\"0x07628b5ffcaa341af16e721832cd4fd211717780\":\"0x82255ac74e1c29e322f76aa1d80f15bd515c601b619b0801ef3d1a1d220741483851dc16a0882ff15433590021d609dbe8cdeec7190eac78c533490b2b43d6a300\",\"0x3de52dba2d878335f8532f95961d33cbce1b1f37\":\"0x9ed769a1ccb7f7728a3e7557c33ee74c26fbb38e86a5711454c52363ec6725a867822cb7e9547cdffe40f70f4044a6b3d282c7a6f8d3d1ee649a1e91f55d6e2700\",\"0xa92ff2bfbde5f31b6e59c21c730ef452ae2c0438\":\"0x599efb45c29651e2b07037de7e2a73ae6453c0797c38088a2ae13a47280d14e51d9ec97da15b7d5d94b9ce5e5b79dcc121c577c9785d2fc386d4f7e2cd8a38b000\"}}},\"proposer\":\"0xa92ff2bfbde5f31b6e59c21c730ef452ae2c0438\",\"gasUsedDec\":0,\"numberDec\":11}"

const citaReceipt = "{\"transactionHash\":\"0x89093750b70d097c7568c67ce24d17bab06e0786d6fa040f9f5cb26d87fcff77\",\"transactionIndex\":0,\"blockHash\":\"0xf39fec56ef73674f48f199832b42a42bde979b53051abfbbee39450103f6b2b2\",\"blockNumber\":753973,\"cumulativeGasUsed\":null,\"cumulativeQuotaUsed\":96160,\"gasUsed\":null,\"quotaUsed\":96160,\"contractAddress\":null,\"root\":null,\"status\":null,\"from\":null,\"to\":null,\"logs\":[{\"removed\":false,\"logIndex\":0,\"transactionIndex\":0,\"transactionHash\":\"0x89093750b70d097c7568c67ce24d17bab06e0786d6fa040f9f5cb26d87fcff77\",\"blockHash\":\"0xf39fec56ef73674f48f199832b42a42bde979b53051abfbbee39450103f6b2b2\",\"blockNumber\":753973,\"address\":\"0xdf8a72a82c146c669a41d3d4c4c3ee215fb629ac\",\"data\":\"0x0000000000000000000000000000000000000000000000000000000000000001\",\"transactionLogIndex\":\"0x0\",\"topics\":[],\"transactionIndexRaw\":\"0x0\",\"logIndexRaw\":\"0x0\",\"blockNumberRaw\":\"0xb8135\"}],\"logsBloom\":\"0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000080000000000000000000000008000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000000000000000000000000000000\",\"errorMessage\":null,\"transactionIndexRaw\":\"0x0\",\"cumulativeGasUsedRaw\":null,\"cumulativeQuotaUsedRaw\":\"0x177a0\",\"gasUsedRaw\":null,\"quotaUsedRaw\":\"0x177a0\",\"blockNumberRaw\":\"0xb8135\"}\n"

func TestVerifyPoc(t *testing.T) {
	header := PocCitaHeader{}
	err := json.Unmarshal([]byte(citaHeader), &header)
	require.Nil(t, err)

	signer := "0x07628b5ffcaa341af16e721832cd4fd211717780"
	signature := header.Proof.Bft.Commits[signer]
	verify, err := asym.Verify(crypto.Secp256k1, hexutil.Decode(signature), hexutil.Decode(header.Proof.Bft.Proposal), *types.NewAddressByStr(signer))
	require.NotNil(t, err)
	require.False(t, verify)

	receipt := TransactionReceipt{}
	err = json.Unmarshal([]byte(citaReceipt), &receipt)
	require.Nil(t, err)

}
