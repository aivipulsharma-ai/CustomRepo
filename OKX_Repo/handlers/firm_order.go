package handlers

import (
	"encoding/json"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/dextr_avs/okx_repo/config"
	"github.com/dextr_avs/okx_repo/models"
	"github.com/dextr_avs/okx_repo/services"
	"github.com/ethereum/go-ethereum/common"
)

type FirmOrderHandler struct {
	cfg    *config.Config
	pricer *services.PricerService
	signer *services.EVMSigner
}

func NewFirmOrderHandler(cfg *config.Config, pricer *services.PricerService, signer *services.EVMSigner) *FirmOrderHandler {
	return &FirmOrderHandler{cfg: cfg, pricer: pricer, signer: signer}
}

func (h *FirmOrderHandler) HandleFirmOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeOKXError(w, http.StatusMethodNotAllowed, "method not allowed", "")
		return
	}

	var req models.FirmOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeOKXError(w, http.StatusBadRequest, "invalid json", err.Error())
		return
	}

	if req.ChainIndex == "" || req.TakerAsset == "" || req.MakerAsset == "" || req.TakerAmount == "" || req.TakerAddress == "" || req.RFQID == 0 {
		writeOKXError(w, http.StatusBadRequest, "missing required fields", "")
		return
	}

	chain, ok := h.cfg.FindChain(req.ChainIndex)
	if !ok {
		writeOKXError(w, http.StatusBadRequest, "unsupported chainIndex", req.ChainIndex)
		return
	}

	// Parse takerAmount (base units).
	takerAmountInt, ok := new(big.Int).SetString(strings.TrimSpace(req.TakerAmount), 10)
	if !ok || takerAmountInt.Sign() <= 0 {
		writeOKXError(w, http.StatusBadRequest, "invalid takerAmount", req.TakerAmount)
		return
	}

	takerTok, ok := h.pricer.FindToken(req.TakerAsset)
	if !ok {
		writeOKXError(w, http.StatusBadRequest, "unknown takerAsset", req.TakerAsset)
		return
	}
	makerTok, ok := h.pricer.FindToken(req.MakerAsset)
	if !ok {
		writeOKXError(w, http.StatusBadRequest, "unknown makerAsset", req.MakerAsset)
		return
	}

	rate, err := h.pricer.TakerTokenRate(req.TakerAsset, req.MakerAsset)
	if err != nil {
		writeOKXError(w, http.StatusBadRequest, "unable to price pair", err.Error())
		return
	}

	// Convert takerAmount (base units) -> token units, then to maker token base units.
	takerAmountTokens := new(big.Rat).SetInt(takerAmountInt)
	takerDiv := new(big.Rat).SetInt(pow10(takerTok.Decimals))
	takerAmountTokens.Quo(takerAmountTokens, takerDiv)

	makerAmountTokens := new(big.Rat).Mul(takerAmountTokens, rate) // maker tokens
	makerMul := new(big.Rat).SetInt(pow10(makerTok.Decimals))
	makerAmountBase := new(big.Rat).Mul(makerAmountTokens, makerMul)

	// Floor(big.Rat) into integer base units: num/den.
	makerAmountInt := new(big.Int).Div(makerAmountBase.Num(), makerAmountBase.Denom())
	if makerAmountInt.Sign() <= 0 {
		writeOKXError(w, http.StatusUnprocessableEntity, "makerAmount computed as zero", "")
		return
	}

	// OKX enforces fixed expiry timing (40s) per docs; we clamp to 40 seconds.
	expirySec := time.Now().Unix() + 40

	order := services.OrderRFQ{
		RFQID:              new(big.Int).SetUint64(req.RFQID),
		Expiry:             big.NewInt(expirySec),
		MakerAsset:         common.HexToAddress(req.MakerAsset),
		TakerAsset:         common.HexToAddress(req.TakerAsset),
		MakerAddress:       common.HexToAddress(h.cfg.Maker.Address),
		MakerAmount:        makerAmountInt,
		TakerAmount:        takerAmountInt,
		UsePermit2:         false,
		Permit2Signature:   []byte{},
		Permit2Witness:     [32]byte{},
		Permit2WitnessType: "",
	}

	sig, err := h.signer.SignOrder(chain, order)
	if err != nil {
		writeOKXError(w, http.StatusInternalServerError, "failed to sign order", err.Error())
		return
	}

	data := models.FirmOrderData{
		ChainIndex:      req.ChainIndex,
		RFQID:           req.RFQID,
		Expiry:          expirySec,
		PMMProtocol:     chain.Settlement,
		MakerAmount:     makerAmountInt.String(),
		MakerAddress:    h.cfg.Maker.Address,
		TakerAsset:      req.TakerAsset,
		MakerAsset:      req.MakerAsset,
		TakerAmount:     req.TakerAmount,
		TakerAddress:    req.TakerAddress,
		Signature:       sig,
		SignatureScheme: string(services.SignatureSchemeEIP712),
		UsePermit2:      false,
	}
	// Back-compat with docs example field name.
	data.SignSchemeLegacyKey = data.SignatureScheme

	resp := models.OKXResponse[models.FirmOrderData]{Code: "0", Msg: "", Data: data}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

