package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum/crypto/kzg4844"
	"github.com/ethereum/go-ethereum/common"
)

func main() {
	// Example blob data (you can replace this with your own)
	// Simple example: "Hello World" padded with zeros
	blobDataHex := "48656c6c6f20576f726c64" // "Hello World" in hex

	fmt.Println("KZG Blob Commitment and Proof Generation PoC")
	fmt.Println(strings.Repeat("=", 50))

	// Option 1: Parse blob data from hex string (current approach)
	// blob, err := createBlobFromHex(blobDataHex)
	
	// Option 2: Load blob data from file
	blob, err := createBlobFromFile("blob_data.txt")
	if err != nil {
		log.Printf("Failed to load from file, falling back to hex string: %v", err)
		// Fall back to hex string if file doesn't exist
		blob, err = createBlobFromHex(blobDataHex)
		if err != nil {
			log.Fatalf("Failed to create blob: %v", err)
		}
	}

	fmt.Printf("Blob data size: %d bytes\n", len(blob[:]))
	fmt.Printf("First 32 bytes: %x\n", blob[:32])

	// Generate KZG commitment
	commitment, err := kzg4844.BlobToCommitment(&blob)
	if err != nil {
		log.Fatalf("Failed to generate KZG commitment: %v", err)
	}

	fmt.Printf("KZG Commitment (48 bytes): %x\n", commitment[:])

	// Generate KZG proof
	proof, err := kzg4844.ComputeBlobProof(&blob, commitment)
	if err != nil {
		log.Fatalf("Failed to generate KZG proof: %v", err)
	}

	fmt.Printf("KZG Proof (48 bytes): %x\n", proof[:])

	// Compute versioned hash (blob hash)
	versionedHash := computeVersionedHash(commitment)
	fmt.Printf("Versioned Hash (blob hash): %x\n", versionedHash[:])

	// Verify the proof
	err = kzg4844.VerifyBlobProof(&blob, commitment, proof)
	if err != nil {
		log.Fatalf("Proof verification failed: %v", err)
	}

	fmt.Println("✅ Proof verification successful!")

	// Display summary
	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Println("SUMMARY:")
	fmt.Printf("• Blob Data: %d bytes\n", len(blob[:]))
	fmt.Printf("• KZG Commitment: %x\n", commitment[:])
	fmt.Printf("• KZG Proof: %x\n", proof[:])
	fmt.Printf("• Versioned Hash: %x\n", versionedHash[:])
	fmt.Println("• Verification: PASSED ✅")
}

// createBlobFromHex creates a KZG blob from hex string, padding to 131072 bytes if needed
func createBlobFromHex(hexStr string) (kzg4844.Blob, error) {
	var blob kzg4844.Blob

	// Remove 0x prefix if present
	if len(hexStr) >= 2 && hexStr[:2] == "0x" {
		hexStr = hexStr[2:]
	}

	// Decode hex string
	data, err := hex.DecodeString(hexStr)
	if err != nil {
		return blob, fmt.Errorf("failed to decode hex string: %w", err)
	}

	// Check if data is too large
	if len(data) > len(blob) {
		return blob, fmt.Errorf("blob data too large: %d bytes, max %d bytes", len(data), len(blob))
	}

	// Copy data to blob (rest will be zero-padded automatically)
	copy(blob[:], data)

	return blob, nil
}

// computeVersionedHash computes the versioned hash from KZG commitment
// createBlobFromFile creates a KZG blob from a file containing hex data
func createBlobFromFile(filename string) (kzg4844.Blob, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return kzg4844.Blob{}, fmt.Errorf("failed to read file: %w", err)
	}
	
	// Remove whitespace and newlines
	hexStr := strings.ReplaceAll(string(data), "\n", "")
	hexStr = strings.ReplaceAll(hexStr, " ", "")
	hexStr = strings.ReplaceAll(hexStr, "\t", "")
	
	return createBlobFromHex(hexStr)
}

// createBlobFromBytes creates a KZG blob from raw bytes
func createBlobFromBytes(data []byte) (kzg4844.Blob, error) {
	var blob kzg4844.Blob
	
	if len(data) > len(blob) {
		return blob, fmt.Errorf("data too large: %d bytes, max %d bytes", len(data), len(blob))
	}
	
	copy(blob[:], data)
	return blob, nil
}

// createBlobFromReader creates a KZG blob from an io.Reader
func createBlobFromReader(r io.Reader) (kzg4844.Blob, error) {
	var blob kzg4844.Blob
	n, err := io.ReadFull(r, blob[:])
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return blob, fmt.Errorf("failed to read data: %w", err)
	}
	
	fmt.Printf("Read %d bytes from reader\n", n)
	return blob, nil
}

// computeVersionedHash computes the versioned hash from KZG commitment
func computeVersionedHash(commitment kzg4844.Commitment) common.Hash {
	hasher := sha256.New()
	return kzg4844.CalcBlobHashV1(hasher, &commitment)
}