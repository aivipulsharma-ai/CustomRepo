package handlers

import (
	"encoding/json"
	"math/big"
	"net/http"
	"strings"

	"github.com/dextr_avs/okx_repo/models"
)

func writeOKXError(w http.ResponseWriter, status int, msg string, details string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(status)
	if details != "" {
		msg = msg + ": " + details
	}
	_ = json.NewEncoder(w).Encode(models.OKXResponse[any]{
		Code: httpStatusToOKXCode(status),
		Msg:  msg,
	})
}

func httpStatusToOKXCode(status int) string {
	// OKX examples use "0" for success; they don't document error codes for MM endpoints.
	// We return a stringified HTTP status for easier debugging.
	return strconv(status)
}

func strconv(i int) string {
	return big.NewInt(int64(i)).String()
}

func ratToDecimalString(r *big.Rat, prec int) string {
	// Render with fixed decimals without scientific notation.
	if r == nil {
		return "0"
	}
	f := new(big.Float).SetPrec(uint(prec * 4)).SetRat(r)
	s := f.Text('f', prec)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	if s == "" || s == "-" {
		return "0"
	}
	return s
}

func pow10(decimals int) *big.Int {
	if decimals <= 0 {
		return big.NewInt(1)
	}
	return new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)
}

