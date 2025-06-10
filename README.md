# KZG Blob Commitment and Proof PoC

This Go project demonstrates how to create KZG commitments and proofs from blob data, similar to how Optimism processes blob transactions (EIP-4844).

## Features

- **Blob Data Parsing**: Convert hex string blob data to KZG blob format (131,072 bytes)
- **KZG Commitment Generation**: Create cryptographic commitments from blob data
- **KZG Proof Generation**: Generate proofs to verify commitment-blob relationships
- **Versioned Hash Computation**: Create blob hashes used in transactions
- **Proof Verification**: Verify that proofs are mathematically correct

## Usage

1. **Install dependencies**:
   ```bash
   go mod tidy
   ```

2. **Run the example**:
   ```bash
   go run main.go
   ```

3. **Use your own blob data**:
   Edit the `blobDataHex` variable in `main.go` with your hex-encoded blob data.

## Example Output

```
KZG Blob Commitment and Proof Generation PoC
==================================================
Blob data size: 131072 bytes
First 32 bytes: 48656c6c6f20576f726c64000000000000000000000000000000000000000000
KZG Commitment (48 bytes): ad43e5d9c8744b1319bea1fcd4438f7faca6ecce16a5ad1047f37dd89bd7841c665e811da6a09bf47d452fc7bd0189d8
KZG Proof (48 bytes): b39aaf5466ac583947d8dc45a96824b56c828f2ccba1594a13ca27d2169b2e03f5049919262cb503ab728db347a6b3bf
Versioned Hash (blob hash): 0105a972a579bb3d58dafa230911abfff4ae900274de07b63c4ad99b8b9269ee
✅ Proof verification successful!
```

## How It Works

1. **Blob Creation**: Raw hex data is parsed and padded to 131,072 bytes (blob size)
2. **KZG Commitment**: Uses `kzg4844.BlobToCommitment()` to create a 48-byte commitment
3. **KZG Proof**: Uses `kzg4844.ComputeBlobProof()` to generate a 48-byte proof
4. **Versioned Hash**: SHA-256 hash of the commitment with version prefix
5. **Verification**: Mathematical verification using `kzg4844.VerifyBlobProof()`

## Dependencies

- `github.com/ethereum/go-ethereum`: For KZG4844 cryptographic functions

## Notes

- Blob data is automatically zero-padded to 131,072 bytes if smaller
- KZG verification uses polynomial mathematics, not data reconstruction
- This matches the implementation used in Optimism's op-batcher and op-service