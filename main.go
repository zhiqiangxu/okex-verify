package main

import (
	"encoding/hex"
	"fmt"

	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/ethclient"
	oksdk "github.com/okex/exchain-go-sdk"
	common1 "github.com/polynetwork/poly/common"
	common2 "github.com/polynetwork/poly/native/service/cross_chain_manager/common"
	"github.com/polynetwork/poly/native/service/cross_chain_manager/eth"
	"github.com/zhiqiangxu/okex-verify/pkg/eccm_abi"
	"github.com/zhiqiangxu/okex-verify/pkg/tools"
)

var (
	rpcURL   = "https://exchaintestrpc.okex.org"
	rpcTMURL = "https://exchaintesttmrpc.okex.org"
	name     = "alice"
	passWd   = "12345678"
	mnemonic = "giggle sibling fun arrow elevator spoon blood grocery laugh tortoise culture tool"
	addr     = "okexchain1ntvyep3suq5z7789g7d5dejwzameu08m6gh7yl"
)

func getProof() {

	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		panic(fmt.Sprintf("ethclient.Dial failed:%v", err))
	}

	receipt, err := client.TransactionReceipt(context.Background(), common.HexToHash("0x05B02D94644BE47727A4B0FEAC3B8552EE6CFA738AB244CDAD8BA18A82ED766C"))
	if err != nil {
		panic(fmt.Sprintf("TransactionReceipt failed:%v", err))
	}
	eccmAddr := common.HexToAddress("0x41B323acDdCe4692E4618978bf67DA189C7692d3")

	eccm, err := eccm_abi.NewEthCrossChainManager(eccmAddr, client)
	if err != nil {
		panic(fmt.Sprintf("eccm_abi.NewEthCrossChainManager failed:%v", err))
	}
	for _, elog := range receipt.Logs {

		if elog.Address == eccmAddr {

			evt, err := eccm.ParseCrossChainEvent(*elog)
			if err != nil {
				panic(fmt.Sprintf("eccm.ParseCrossChainEvent failed:%v", err))
			}

			param := &common2.MakeTxParam{}
			err = param.Deserialization(common1.NewZeroCopySource([]byte(evt.Rawdata)))
			if err != nil {
				panic(fmt.Sprintf("param.Deserialization failed:%v", err))
			}

			txIDBig := big.NewInt(0)
			txIDBig.SetBytes(evt.TxId)
			txID := tools.EncodeBigInt(txIDBig)
			// txHash := evt.Raw.TxHash.Bytes()

			keyBytes, err := eth.MappingKeyAt(txID, "01")
			if err != nil {
				panic(fmt.Sprintf("eth.MappingKeyAt failed:%v", err))
			}

			refHeight, err := client.BlockNumber(context.Background())
			if err != nil {
				panic(fmt.Sprintf("client.BlockNumber failed:%v", err))
			}
			height := int64(refHeight - 3)
			heightHex := hexutil.EncodeBig(big.NewInt(height))
			proofKey := hexutil.Encode(keyBytes)

			proof, err := tools.GetProof(rpcURL, "0x2a88feB48E176b535da78266990D556E588Cfe06", proofKey, heightHex)
			if err != nil {
				panic(fmt.Sprintf("tools.GetProof failed:%v", err))
			}

			fmt.Println("proof", hex.EncodeToString(proof))

		}
	}
}

func main() {

	getProof()
	return

	config, _ := oksdk.NewClientConfig(rpcURL, "okexchain-65", oksdk.BroadcastBlock, "0.01okt", 200000, 0, "")
	client := oksdk.NewClient(config)

	height := int64(2012155)
	commitResult, err := client.Tendermint().QueryCommitResult(height)
	if err != nil {
		panic(err)
	}

	valResult, err := client.Tendermint().QueryValidatorsResult(height)
	if err != nil {
		panic(err)
	}

	fmt.Println("commitResult", commitResult)
	fmt.Println("valResult", valResult)
}
