package services

import (
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/dextr_avs/okx_repo/config"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

type SignatureScheme string

const (
	SignatureSchemeEIP712 SignatureScheme = "EIP-712"
)

// OrderRFQ matches the OKX EVM signature docs.
type OrderRFQ struct {
	RFQID             *big.Int
	Expiry            *big.Int
	MakerAsset        common.Address
	TakerAsset        common.Address
	MakerAddress      common.Address
	MakerAmount       *big.Int
	TakerAmount       *big.Int
	UsePermit2        bool
	Permit2Signature  []byte
	Permit2Witness    [32]byte
	Permit2WitnessType string
}

type EVMSigner struct {
	cfg *config.Config
}

func NewEVMSigner(cfg *config.Config) *EVMSigner {
	return &EVMSigner{cfg: cfg}
}

func (s *EVMSigner) SignOrder(chain config.ChainConfig, order OrderRFQ) (string, error) {
	pk, err := parsePrivateKeyHex(s.cfg.Maker.PrivateKey)
	if err != nil {
		return "", err
	}

	domainSeparator, err := s.domainSeparator(chain)
	if err != nil {
		return "", err
	}
	structHash, err := hashOrderRFQ(order)
	if err != nil {
		return "", err
	}

	// digest = keccak256("\x19\x01" || domainSeparator || structHash)
	digest := crypto.Keccak256Hash(append(append([]byte{0x19, 0x01}, domainSeparator.Bytes()...), structHash.Bytes()...))
	sig, err := crypto.Sign(digest.Bytes(), pk)
	if err != nil {
		return "", err
	}
	// Normalize V to 27/28 like the 1inch service does.
	if sig[64] < 27 {
		sig[64] += 27
	}
	return "0x" + hex.EncodeToString(sig), nil
}

func (s *EVMSigner) domainSeparator(chain config.ChainConfig) (common.Hash, error) {
	verifying := common.HexToAddress(chain.Settlement)

	// keccak256("EIP712Domain(string name,string version,uint256 chainId,address verifyingContract)")
	domainTypeHash := crypto.Keccak256Hash([]byte("EIP712Domain(string name,string version,uint256 chainId,address verifyingContract)"))
	nameHash := crypto.Keccak256Hash([]byte("OnChain Labs PMM Protocol"))
	versionHash := crypto.Keccak256Hash([]byte("1.0"))

	args := abi.Arguments{
		{Type: mustType("bytes32")},
		{Type: mustType("bytes32")},
		{Type: mustType("bytes32")},
		{Type: mustType("uint256")},
		{Type: mustType("address")},
	}
	encoded, err := args.Pack(domainTypeHash, nameHash, versionHash, big.NewInt(chain.ChainID), verifying)
	if err != nil {
		return common.Hash{}, err
	}
	return crypto.Keccak256Hash(encoded), nil
}

func hashOrderRFQ(o OrderRFQ) (common.Hash, error) {
	// Must match Solidity type string in OKX docs EXACTLY.
	typeStr := "OrderRFQ(uint256 rfqId,uint256 expiry,address makerAsset,address takerAsset,address makerAddress,uint256 makerAmount,uint256 takerAmount,bool usePermit2,bytes permit2Signature,bytes32 permit2Witness,string permit2WitnessType)"
	typeHash := crypto.Keccak256Hash([]byte(typeStr))

	permit2SigHash := crypto.Keccak256Hash(o.Permit2Signature)
	witnessTypeHash := crypto.Keccak256Hash([]byte(o.Permit2WitnessType))

	args := abi.Arguments{
		{Type: mustType("bytes32")},
		{Type: mustType("uint256")},
		{Type: mustType("uint256")},
		{Type: mustType("address")},
		{Type: mustType("address")},
		{Type: mustType("address")},
		{Type: mustType("uint256")},
		{Type: mustType("uint256")},
		{Type: mustType("bool")},
		{Type: mustType("bytes32")},
		{Type: mustType("bytes32")},
		{Type: mustType("bytes32")},
	}
	encoded, err := args.Pack(
		typeHash,
		o.RFQID,
		o.Expiry,
		o.MakerAsset,
		o.TakerAsset,
		o.MakerAddress,
		o.MakerAmount,
		o.TakerAmount,
		o.UsePermit2,
		permit2SigHash,
		o.Permit2Witness,
		witnessTypeHash,
	)
	if err != nil {
		return common.Hash{}, err
	}
	return crypto.Keccak256Hash(encoded), nil
}

func parsePrivateKeyHex(key string) (*ecdsa.PrivateKey, error) {
	key = strings.TrimSpace(key)
	key = strings.TrimPrefix(key, "0x")
	if key == "" {
		return nil, errors.New("empty private key")
	}
	b, err := hex.DecodeString(key)
	if err != nil {
		return nil, fmt.Errorf("invalid private key hex: %w", err)
	}
	return crypto.ToECDSA(b)
}

func mustType(t string) abi.Type {
	ty, err := abi.NewType(t, "", nil)
	if err != nil {
		panic(err)
	}
	return ty
}

