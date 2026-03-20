package models

// OKX schema: code/msg/data wrapper.
type OKXResponse[T any] struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
	Data T      `json:"data,omitempty"`
}

type PricingData struct {
	ChainIndex string      `json:"chainIndex"`
	LevelData  []LevelData `json:"levelData"`
}

type LevelData struct {
	TakerTokenAddress string      `json:"takerTokenAddress"`
	MakerTokenAddress string      `json:"makerTokenAddress"`
	Levels            [][2]string `json:"levels"`
}

type FirmOrderRequest struct {
	ChainIndex         string `json:"chainIndex"`
	TakerAsset         string `json:"takerAsset"`
	MakerAsset         string `json:"makerAsset"`
	TakerAmount        string `json:"takerAmount"`
	TakerAddress       string `json:"takerAddress"`
	RFQID              uint64 `json:"rfqId"`
	ExpiryDuration     int64  `json:"expiryDuration"`
	CallData           string `json:"callData,omitempty"`
	BeneficiaryAddress string `json:"beneficiaryAddress,omitempty"`
}

type FirmOrderData struct {
	ChainIndex          string `json:"chainIndex"`
	RFQID               uint64 `json:"rfqId"`
	Expiry              int64  `json:"expiry"`
	PMMProtocol         string `json:"pmmProtocol,omitempty"`
	MakerAmount         string `json:"makerAmount"`
	MakerAddress        string `json:"makerAddress"`
	TakerAsset          string `json:"takerAsset"`
	MakerAsset          string `json:"makerAsset"`
	TakerAmount         string `json:"takerAmount"`
	TakerAddress        string `json:"takerAddress"`
	Signature           string `json:"signature"`
	SignatureScheme     string `json:"signatureScheme,omitempty"`
	UsePermit2          bool   `json:"usePermit2,omitempty"`
	Permit2Signature    string `json:"permit2Signature,omitempty"`
	Permit2Witness      string `json:"permit2Witness,omitempty"`
	Permit2WitnessType  string `json:"permit2WitnessType,omitempty"`
	CallData            string `json:"callData,omitempty"`
	SignSchemeLegacyKey string `json:"sign_scheme,omitempty"`
}

type ErrorResponse struct {
	Code    string `json:"code"`
	Msg     string `json:"msg"`
	Details string `json:"details,omitempty"`
}

