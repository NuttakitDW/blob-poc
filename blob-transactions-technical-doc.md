# Blob Transaction Processing in Optimism (OP Stack)

This document provides a detailed technical overview of how blob transactions (EIP-4844) are processed in the Optimism codebase, with specific file paths and line numbers for reference.

## 1. Blob Transaction Construction and Posting in L2

### Primary Components

The **op-batcher** is the primary component responsible for constructing and posting blob transactions to L1. It works in conjunction with the **op-service/txmgr** transaction manager.

### Blob Transaction Flow

#### Channel Configuration (`op-batcher/batcher/channel_config.go`)
```go
// Line 49-51: Blob configuration
type ChannelConfig struct {
    UseBlobs bool // Whether to use blob transactions
    // ...
}

// Line 95-100: Max frames per blob tx
func (cc *ChannelConfig) MaxFramesPerTx() int {
    if cc.UseBlobs {
        return cc.TargetNumFrames
    }
    return 1
}
```

#### Channel Data Preparation (`op-batcher/batcher/channel.go:L128-146`)
```go
// NextTxData prepares transaction data with blob support
func (c *channel) NextTxData() txData {
    nf := c.cfg.MaxFramesPerTx()
    txdata := txData{frames: make([]frameData, 0, nf), asBlob: c.cfg.UseBlobs}
    for i := 0; i < nf && c.channelBuilder.HasPendingFrame(); i++ {
        frame := c.channelBuilder.NextFrame()
        txdata.frames = append(txdata.frames, frame)
    }
    // ...
}
```

#### Blob Creation (`op-batcher/batcher/tx_data.go:L46-56`)
```go
func (txd *txData) Blobs() ([]*eth.Blob, error) {
    var blobs []*eth.Blob
    for _, frame := range txd.frames {
        if len(frame.data) > eth.MaxBlobDataSize {
            return nil, fmt.Errorf("frame size %d exceeds max blob size %d", len(frame.data), eth.MaxBlobDataSize)
        }
        blob, err := eth.NewBlob(frame.data)
        if err != nil {
            return nil, err
        }
        blobs = append(blobs, blob)
    }
    return blobs, nil
}
```

### Transaction Manager Integration (`op-service/txmgr/txmgr.go`)

#### Blob Transaction Creation (`op-service/txmgr/txmgr.go:L360-415`)
```go
// Line 360-369: Sidecar preparation
var sidecar *types.BlobTxSidecar
var blobHashes []common.Hash
if len(candidate.Blobs) > 0 {
    if candidate.To == nil {
        return nil, errors.New("blob txs cannot deploy contracts")
    }
    if sidecar, blobHashes, err = MakeSidecar(candidate.Blobs); err != nil {
        return nil, fmt.Errorf("failed to make sidecar: %w", err)
    }
}

// Line 400-415: BlobTx type creation
if sidecar != nil {
    if blobBaseFee == nil {
        return nil, errors.New("expected non-nil blobBaseFee")
    }
    blobFeeCap := m.calcBlobFeeCap(blobBaseFee)
    message := &types.BlobTx{
        To:         *candidate.To,
        Data:       candidate.TxData,
        Gas:        gasLimit,
        BlobHashes: blobHashes,
        Sidecar:    sidecar,
    }
    // ...
}
```

## 2. KZG Commitment Generation

The KZG commitment is generated using the `kzg4844` package from go-ethereum. The key function is located in:

### MakeSidecar Function (`op-service/txmgr/txmgr.go:L486-505`)
```go
func MakeSidecar(blobs []*eth.Blob) (*types.BlobTxSidecar, []common.Hash, error) {
    sidecar := &types.BlobTxSidecar{}
    blobHashes := make([]common.Hash, 0, len(blobs))
    for i, blob := range blobs {
        rawBlob := blob.KZGBlob()
        sidecar.Blobs = append(sidecar.Blobs, *rawBlob)
        
        // Line 492: Generate KZG commitment
        commitment, err := kzg4844.BlobToCommitment(rawBlob)
        if err != nil {
            return nil, nil, fmt.Errorf("cannot compute KZG commitment of blob %d in tx candidate: %w", i, err)
        }
        sidecar.Commitments = append(sidecar.Commitments, commitment)
        
        // Line 497: Generate KZG proof (see next section)
        proof, err := kzg4844.ComputeBlobProof(rawBlob, commitment)
        // ...
        
        // Line 502: Convert commitment to versioned hash
        blobHashes = append(blobHashes, eth.KZGToVersionedHash(commitment))
    }
    return sidecar, blobHashes, nil
}
```

### Versioned Hash Computation (`op-service/eth/blob.go:L68-72`)
```go
// KZGToVersionedHash computes the "blob hash" (a.k.a. versioned-hash) of a blob-commitment
func KZGToVersionedHash(commitment kzg4844.Commitment) (out common.Hash) {
    hasher := sha256.New()
    return kzg4844.CalcBlobHashV1(hasher, &commitment)
}
```

## 3. KZG Proof Generation

The KZG proof is generated alongside the commitment in the `MakeSidecar` function:

### Proof Generation (`op-service/txmgr/txmgr.go:L497-501`)
```go
// Line 497: Generate KZG proof for blob verification
proof, err := kzg4844.ComputeBlobProof(rawBlob, commitment)
if err != nil {
    return nil, nil, fmt.Errorf("cannot compute KZG proof for fast commitment verification of blob %d in tx candidate: %w", i, err)
}
sidecar.Proofs = append(sidecar.Proofs, proof)
```

**Important**: Optimism relies on the go-ethereum `kzg4844` package for proof generation. The actual proof computation is handled by the underlying cryptographic library, not implemented directly in Optimism.

## 4. Commitment and Proof Verification on Ethereum L1

### Blob Data Retrieval (`op-node/rollup/derive/blob_data_source.go`)

The op-node retrieves and processes blob data from L1:

#### Blob Hash Extraction (`op-node/rollup/derive/blob_data_source.go:L118-149`)
```go
func dataAndHashesFromTxs(txs types.Transactions, config *DataSourceConfig, batcherAddr common.Address, logger log.Logger) ([]blobOrCalldata, []eth.IndexedBlobHash) {
    // ...
    for _, tx := range txs {
        // Line 129: Check if transaction is Type 3 (blob)
        if tx.Type() != types.BlobTxType {
            calldata := eth.Data(tx.Data())
            data = append(data, blobOrCalldata{nil, &calldata})
            continue
        }
        
        // Line 138-146: Extract blob hashes from Type 3 transactions
        for _, h := range tx.BlobHashes() {
            idh := eth.IndexedBlobHash{
                Index: uint64(blobIndex),
                Hash:  h,
            }
            hashes = append(hashes, idh)
            data = append(data, blobOrCalldata{nil, nil})
            blobIndex += 1
        }
    }
    return data, hashes
}
```

#### Blob Fetching (`op-node/rollup/derive/blob_data_source.go:L80-113`)
```go
func (ds *BlobDataSource) open() ([]blobOrCalldata, error) {
    // Line 92-94: Fetch blobs from L1 beacon client
    blobs, err := ds.fetcher.GetBlobs(ds.ctx, ds.ref, hashes)
    if err != nil {
        return nil, NewResetError(fmt.Errorf("failed to fetch blobs: %w", err))
    }
    
    // Verify and fill blob data
    if err := fillBlobPointers(data, blobs); err != nil {
        return nil, err
    }
    // ...
}
```

### Op-program Blob Verification

The op-program includes specialized blob handling for fault proofs:

#### Blob Fetcher (`op-program/client/l1/blob_fetcher.go:L14-36`)
```go
type BlobFetcher struct {
    L1Beacon L1BeaconClient
}

func (l *BlobFetcher) GetBlobs(ctx context.Context, ref eth.L1BlockRef, hashes []eth.IndexedBlobHash) ([]*eth.Blob, error) {
    if len(hashes) == 0 {
        return []*eth.Blob{}, nil
    }
    return l.L1Beacon.GetBlobs(ctx, ref, hashes)
}
```

#### Blob Reconstruction from Field Elements (`op-program/client/l1/oracle.go:L102-124`)
```go
func (p *PreimageOracle) GetBlob(ref eth.L1BlockRef, blobHash eth.IndexedBlobHash) *eth.Blob {
    // Line 109: Get commitment from preimage oracle
    commitment := p.oracle.Get(preimage.Sha256Key(blobHash.Hash))
    
    // Line 111-121: Reconstruct blob from 4096 field elements
    blob := eth.Blob{}
    fieldElemKey := make([]byte, 80)
    copy(fieldElemKey[:48], commitment)
    for i := 0; i < params.BlobTxFieldElementsPerBlob; i++ {
        rootOfUnity := RootsOfUnity[i].Bytes()
        copy(fieldElemKey[48:], rootOfUnity[:])
        fieldElement := p.oracle.Get(preimage.BlobKey(crypto.Keccak256(fieldElemKey)))
        copy(blob[i<<5:(i+1)<<5], fieldElement[:])
    }
    
    return &blob
}
```

### Verification Process

1. **Execution Layer**: The execution client verifies that blob versioned hashes in the transaction match the commitments in the blob sidecar.

2. **Beacon Layer**: The beacon node:
   - Stores blob sidecars containing the full blob data, commitments, and proofs
   - Provides blob data via the beacon API when requested
   - Ensures blob availability for the required retention period

3. **Blob Verification** (`op-service/eth/blob.go:L76-78`):
```go
func VerifyBlobProof(blob *Blob, commitment kzg4844.Commitment, proof kzg4844.Proof) error {
    return kzg4844.VerifyBlobProof(blob.KZGBlob(), commitment, proof)
}
```

## 5. Configuration and Constants

### Blob Constants (`op-service/eth/blob.go:L19-24`)
```go
const (
    BlobSize         = 4096 * 32  // 131072 bytes
    MaxBlobDataSize  = (4096*31 - 4) * 32 // 127,228 bytes of calldata per blob
    EncodingVersion  = 0
    VersionOffset    = 1  // offset of the version byte in the blob encoding
    Rounds           = 1024 // number of encode/decode rounds for test vectors
)
```

### Blob Encoding (`op-service/eth/blob.go:L92-191`)
The `FromData` function implements the blob encoding algorithm that converts raw data into the blob format, ensuring proper field element constraints for KZG commitments.

## Summary

The Optimism implementation of blob transactions follows the EIP-4844 specification:

1. **Construction**: The op-batcher constructs blob transactions when `UseBlobs` is enabled, packaging multiple frames into blobs.

2. **Commitment & Proof**: The transaction manager generates KZG commitments and proofs using go-ethereum's `kzg4844` package.

3. **Submission**: Blob transactions are submitted to L1 with the blob sidecar containing the data, commitments, and proofs.

4. **Retrieval**: The op-node fetches blobs from the L1 beacon API using the blob hashes from Type 3 transactions.

5. **Verification**: Verification happens at multiple levels - the execution layer checks versioned hashes, while the beacon layer maintains and serves the full blob data with proofs.

This architecture ensures data availability while reducing L1 calldata costs through the efficient blob transaction mechanism introduced in EIP-4844.