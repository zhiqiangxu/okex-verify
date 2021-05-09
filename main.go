package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"

	"github.com/okex/exchain/app"
	"github.com/okex/exchain/app/codec"

	"github.com/zhiqiangxu/okex-verify/pkg/eccm_abi"

	"github.com/cosmos/cosmos-sdk/store/rootmulti"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/light"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie"
	proto "github.com/gogo/protobuf/proto"
	oksdk "github.com/okex/exchain-go-sdk"
	common1 "github.com/polynetwork/poly/common"
	common2 "github.com/polynetwork/poly/native/service/cross_chain_manager/common"
	"github.com/polynetwork/poly/native/service/cross_chain_manager/eth"
	"github.com/tendermint/tendermint/crypto/merkle"
	"github.com/tendermint/tendermint/types"
	"github.com/zhiqiangxu/okex-verify/pkg/tools"
)

var (
	rpcURL   = "http://18.167.77.79:26659"
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

			fmt.Println("txIDBig", txIDBig, "storage key", hex.EncodeToString(keyBytes))

			refHeight, err := client.BlockNumber(context.Background())
			if err != nil {
				panic(fmt.Sprintf("client.BlockNumber failed:%v", err))
			}
			height := int64(refHeight)
			heightHex := hexutil.EncodeBig(big.NewInt(height))
			proofKey := hexutil.Encode(keyBytes)
			proofKey = "/evm/xxx"

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

			blockData, err := client.HeaderByNumber(context.Background(), big.NewInt(height+1))
			if err != nil {
				panic(fmt.Sprintf("HeaderByNumber failed:%v", err))
			}

			var mproof merkle.Proof
			err = proto.UnmarshalText(okProof.StorageProofs[0].Proof[0], &mproof)
			if err != nil {
				panic(fmt.Sprintf("proto.UnmarshalText failed:%v", err))
			}

			fmt.Println("proof", string(proof))
			// var StorageResult

			keyPath := "/"
			for i := range mproof.Ops {
				op := mproof.Ops[len(mproof.Ops)-1-i]
				keyPath += string(op.Key)
				keyPath += "/"
			}

			keyPath = strings.TrimSuffix(keyPath, "/")
			//keyPath = "/evm/\005\r\002\035\020\253\236\025_\301\350p]\022\267?\233\323\336\n6ZF\205\027\326\370?\256pe\2520`\374\n\337\n\304\260\301H\026GJhw\215\220w;\323q"

			fmt.Println("keyPath", keyPath)

			prt := rootmulti.DefaultProofRuntime()
			err = prt.VerifyAbsence(&mproof, blockData.Root.Bytes(), keyPath)
			if err != nil {
				panic(fmt.Sprintf("VerifyAbsence failed:%v", err))
			}

			fmt.Println("proof ok")
			// eccdBytes := common.FromHex(eccd)
			// result, err := verifyMerkleProof(okProof, blockData, eccdBytes)
			// if err != nil {
			// 	panic(fmt.Sprintf("verifyMerkleProof failed:%v, proof:%v", err, string(proof)))
			// }
			// fmt.Println("result", string(result))

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

	addr := common.Hex2Bytes(common2.Replace0x(okProof.Address.String()))
	if !bytes.Equal(addr, contractAddr) {
		return nil, fmt.Errorf("verifyMerkleProof, contract address is error, proof address: %s, side chain address: %s", okProof.Address, hex.EncodeToString(contractAddr))
	}
	acctKey := crypto.Keccak256(addr)

	fmt.Println("height", blockData.Number, "blockData.Root", blockData.Root.Hex())
	//2. verify account proof
	acctVal, err := trie.VerifyProof(blockData.Root, acctKey, ns)
	if err != nil {
		return nil, fmt.Errorf("verifyMerkleProof, verify account proof error:%s", err)
	}

	nounce := new(big.Int)
	_, ok := nounce.SetString(common2.Replace0x(okProof.Nonce.String()), 16)
	if !ok {
		return nil, fmt.Errorf("verifyMerkleProof, invalid format of nounce:%s", okProof.Nonce)
	}

	balance := new(big.Int)
	_, ok = balance.SetString(common2.Replace0x(okProof.Balance.String()), 16)
	if !ok {
		return nil, fmt.Errorf("verifyMerkleProof, invalid format of balance:%s", okProof.Balance)
	}

	storageHash := common.HexToHash(common2.Replace0x(okProof.StorageHash.Hex()))
	codeHash := common.HexToHash(common2.Replace0x(okProof.CodeHash.Hex()))

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

	config, _ := oksdk.NewClientConfig(rpcTMURL, "okexchain-65", oksdk.BroadcastBlock, "0.01okt", 200000, 0, "")
	client := oksdk.NewClient(config)
	// result, err := client.Tendermint().QueryTxResult("05B02D94644BE47727A4B0FEAC3B8552EE6CFA738AB244CDAD8BA18A82ED766C", true)
	// if err != nil {
	// 	panic(fmt.Sprintf("QueryTxResult failed:%v", err))
	// }

	// resultBytes, _ := json.Marshal(result)

	// fmt.Println("result", string(resultBytes))
	// return

	height := int64(2012155)

	block, err := client.Tendermint().QueryBlock(height)
	if err != nil {
		panic(err)
	}
	commitResult, err := client.Tendermint().QueryCommitResult(height)
	if err != nil {
		panic(err)
	}

	valResult, err := client.Tendermint().QueryValidatorsResult(height)
	if err != nil {
		panic(err)
	}

	hdr := CosmosHeader{Header: block.Header, Commit: block.LastCommit, Valsets: valResult.Validators}

	cdc := codec.MakeCodec(app.ModuleBasics)
	hdrBytes, err := cdc.MarshalJSON(hdr)
	if err != nil {
		panic(err)
	}

	err = cdc.UnmarshalJSON(hdrBytes, &hdr)
	if err != nil {
		panic(err)
	}

	fmt.Println("block", block)
	fmt.Println("commitResult", commitResult)
	fmt.Println("valResult", valResult)
}

type CosmosHeader struct {
	Header  types.Header
	Commit  *types.Commit
	Valsets []*types.Validator
}
