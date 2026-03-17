# Dextr AVS - Token Swap Actively Validated Service

Dextr AVS is an Actively Validated Service (AVS) built with the Catalyst SDK that handles token swap operations. It provides a decentralized, consensus-driven approach to token swapping with path finding and validation capabilities.

## Features

- **Multi-hop Token Swaps**: Find optimal paths between any two tokens through intermediate vaults
- **Path Finding Algorithm**: Uses BFS to discover valid swap routes
- **Consensus Validation**: All swap operations are validated through the Catalyst consensus mechanism
- **HTTP API**: RESTful endpoints for swap operations
- **CLI Interface**: Command-line tools for interacting with the service

## Architecture

The Dextr AVS follows the standard Catalyst SDK architecture:

```
dextr-avs/
├── avs/           # Main AVS implementation
├── cmd/           # CLI commands
├── handlers/      # HTTP and task handlers
├── types/         # Shared types and constants
├── utils/         # Utility functions and validation
└── main.go        # Application entry point
```

## Installation

1. Clone the repository:
```bash
git clone <repository-url>
cd dextr-avs
```

2. Install dependencies:
```bash
make deps
```

3. Build the application:
```bash
make build
```

## Usage

### Starting the AVS

1. Initialize configuration:
```bash
./dextr-app config init
```

2. Start the AVS:
```bash
./dextr-app startapp
```

### Performing Token Swaps

Use the CLI to perform token swaps:

```bash
./dextr-app swap BTC USDC 2.5 12345
```

This command:
- Swaps 2.5 BTC for USDC
- Uses transaction ID 12345
- Finds the optimal path through available vaults

### HTTP API

The AVS exposes an HTTP API for programmatic access:

```bash
curl -X POST http://localhost:8080/swap/12345 \
  -H "Content-Type: application/json" \
  -d '{
    "input_token": "BTC",
    "output_token": "USDC", 
    "amount": 2.5
  }'
```

## Supported Token Pairs

The AVS currently supports the following vault configurations:

- ETH/USDC
- ETH/BNB  
- BNB/USDC
- BTC/ETH
- BTC/BNB

## Development

### Running Tests

```bash
make test
```

### Code Formatting

```bash
make fmt
```

### Linting

```bash
make lint
```

### Building for Different Platforms

```bash
make build-all
```

## Configuration

The AVS uses the standard Catalyst SDK configuration format. Configuration files are stored in `$HOME/.catalyst/config.json`.

Key configuration options:
- `P2PConfig.TCPListenAddr`: P2P listening address
- `KeyStoreConfig.KeyDir`: Directory for cryptographic keys
- `HTTPPort`: HTTP server port

## Consensus Mechanism

The Dextr AVS integrates with the Catalyst consensus network to ensure:

1. **Task Distribution**: Swap requests are distributed across the network
2. **Execution**: Multiple nodes execute the same swap calculation
3. **Verification**: Results are verified through consensus
4. **Finalization**: Only consensus-approved swaps are finalized

## Token Swap Algorithm

The swap algorithm works as follows:

1. **Path Discovery**: Uses BFS to find valid paths between input and output tokens
2. **Validation**: Validates the path for continuity and feasibility
3. **Execution**: Simulates the swap through the path
4. **Consensus**: Results are submitted to the consensus network for validation

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests for new functionality
5. Submit a pull request

## License

[Add your license information here]

## Support

For support and questions, please open an issue in the repository.