package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/light"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie"
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

			fmt.Println("txIDBig", txIDBig)
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

			eccd := "0x2a88feB48E176b535da78266990D556E588Cfe06"
			proof, err := tools.GetProof(rpcURL, eccd, proofKey, heightHex)
			if err != nil {
				panic(fmt.Sprintf("tools.GetProof failed:%v", err))
			}

			okProof := new(tools.ETHProof)
			err = json.Unmarshal(proof, okProof)
			if err != nil {
				panic(fmt.Sprintf("ETHProof Unmarshal failed:%v", err))
			}

			blockData, err := client.HeaderByNumber(context.Background(), big.NewInt(height))
			if err != nil {
				panic(fmt.Sprintf("HeaderByNumber failed:%v", err))
			}

			eccdBytes := common.FromHex(eccd)
			result, err := verifyMerkleProof(okProof, blockData, eccdBytes)
			if err != nil {
				panic(fmt.Sprintf("verifyMerkleProof failed:%v", err))
			}
			fmt.Println("result", string(result))

		}
	}
}

// ProofAccount ...
type ProofAccount struct {
	Nounce   *big.Int
	Balance  *big.Int
	Storage  common.Hash
	Codehash common.Hash
}

func verifyMerkleProof(okProof *tools.ETHProof, blockData *ethtypes.Header, contractAddr []byte) ([]byte, error) {
	//1. prepare verify account
	nodeList := new(light.NodeList)

	for _, s := range okProof.AccountProof {
		p := common2.Replace0x(s)
		nodeList.Put(nil, common.Hex2Bytes(p))
	}
	ns := nodeList.NodeSet()

	addr := common.Hex2Bytes(common2.Replace0x(okProof.Address))
	if !bytes.Equal(addr, contractAddr) {
		return nil, fmt.Errorf("verifyMerkleProof, contract address is error, proof address: %s, side chain address: %s", okProof.Address, hex.EncodeToString(contractAddr))
	}
	acctKey := crypto.Keccak256(addr)

	fmt.Println("blockData.Root", blockData.Root.Hex())
	//2. verify account proof
	acctVal, err := trie.VerifyProof(blockData.Root, acctKey, ns)
	if err != nil {
		return nil, fmt.Errorf("verifyMerkleProof, verify account proof error:%s", err)
	}

	nounce := new(big.Int)
	_, ok := nounce.SetString(common2.Replace0x(okProof.Nonce), 16)
	if !ok {
		return nil, fmt.Errorf("verifyMerkleProof, invalid format of nounce:%s", okProof.Nonce)
	}

	balance := new(big.Int)
	_, ok = balance.SetString(common2.Replace0x(okProof.Balance), 16)
	if !ok {
		return nil, fmt.Errorf("verifyMerkleProof, invalid format of balance:%s", okProof.Balance)
	}

	storageHash := common.HexToHash(common2.Replace0x(okProof.StorageHash))
	codeHash := common.HexToHash(common2.Replace0x(okProof.CodeHash))

	acct := &ProofAccount{
		Nounce:   nounce,
		Balance:  balance,
		Storage:  storageHash,
		Codehash: codeHash,
	}

	acctrlp, err := rlp.EncodeToBytes(acct)
	if err != nil {
		return nil, err
	}

	if !bytes.Equal(acctrlp, acctVal) {
		return nil, fmt.Errorf("verifyMerkleProof, verify account proof failed, wanted:%v, get:%v", acctrlp, acctVal)
	}

	//3.verify storage proof
	nodeList = new(light.NodeList)
	if len(okProof.StorageProofs) != 1 {
		return nil, fmt.Errorf("verifyMerkleProof, invalid storage proof format")
	}

	sp := okProof.StorageProofs[0]
	storageKey := crypto.Keccak256(common.HexToHash(common2.Replace0x(sp.Key)).Bytes())

	for _, prf := range sp.Proof {
		nodeList.Put(nil, common.Hex2Bytes(common2.Replace0x(prf)))
	}

	ns = nodeList.NodeSet()
	val, err := trie.VerifyProof(storageHash, storageKey, ns)
	if err != nil {
		return nil, fmt.Errorf("verifyMerkleProof, verify storage proof error:%s", err)
	}

	return val, nil
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
