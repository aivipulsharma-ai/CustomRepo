# OKX RFQ Market Maker (reference implementation)

This folder contains a standalone Go service implementing OKX DEX **Market Maker Integration** endpoints:

- `GET  /OKXDEX/rfq/pricing?chainIndex=<chain>` (stream price levels)
- `POST /OKXDEX/rfq/firm-order` (return firm order + maker signature)
- `GET  /health`

It mirrors the **folder layout** of the existing `price-feeder/` (1inch RFQ) service, but implements the OKX request/response schema and EIP-712 signing described in the OKX docs.

## Running

From `OKX_Repo/`:

```bash
go mod tidy
go run .
```

## Configuration

The service loads `config.json` (default: `config.json` in the working directory). You can also override key fields via environment variables.

### Env vars

- `CONFIG_FILE` (default: `config.json`)
- `HOST` (default: `0.0.0.0`)
- `PORT` (default: `8081`)
- `LOG_LEVEL` (default: `info`)
- `X_API_KEY` (required for authenticated endpoints)
- `MAKER_ADDRESS` (required, EVM address)
- `MAKER_PRIVATE_KEY` (required, hex private key, with or without `0x`)

## Notes

- **Auth**: OKX will pass your API key as `X-API-KEY` (per docs). This service enforces it on `/OKXDEX/rfq/pricing` and `/OKXDEX/rfq/firm-order`.
- **Signing (EVM)**: Implements the EIP-712 `OrderRFQ` signature (domain `OnChain Labs PMM Protocol`, version `1.0`) as described in the OKX “EVM Signature” section.
- **Chains**: Maps `chainIndex` → `chainId` + settlement `pmmProtocol` using the addresses from the OKX docs.

