package file

import (
	"testing"
)

// BenchmarkSerializeFileRequest benchmarks file request serialization.
func BenchmarkSerializeFileRequest(b *testing.B) {
	benchmarks := []struct {
		name     string
		fileID   uint32
		fileName string
		fileSize uint64
	}{
		{"short_name", 1, "test.txt", testFileSize1KB},
		{"medium_name", 2, "some_longer_filename_for_testing.doc", 1048576},
		{"max_length_name", 3, string(make([]byte, MaxFileNameLength)), testFileSize1GB},
	}

	for _, bm := range benchmarks {
		// Fill with valid characters for max length name
		fileName := bm.fileName
		if len(fileName) > 50 {
			nameBytes := make([]byte, len(fileName))
			for i := range nameBytes {
				nameBytes[i] = byte('a' + (i % 26))
			}
			fileName = string(nameBytes)
		}

		b.Run(bm.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = serializeFileRequest(bm.fileID, fileName, bm.fileSize)
			}
		})
	}
}

// BenchmarkDeserializeFileRequest benchmarks file request deserialization.
func BenchmarkDeserializeFileRequest(b *testing.B) {
	benchmarks := []struct {
		name     string
		fileID   uint32
		fileName string
		fileSize uint64
	}{
		{"short_name", 1, "test.txt", testFileSize1KB},
		{"medium_name", 2, "some_longer_filename_for_testing.doc", 1048576},
		{"max_length_name", 3, string(make([]byte, MaxFileNameLength)), testFileSize1GB},
	}

	for _, bm := range benchmarks {
		// Fill with valid characters for max length name
		fileName := bm.fileName
		if len(fileName) > 50 {
			nameBytes := make([]byte, len(fileName))
			for i := range nameBytes {
				nameBytes[i] = byte('a' + (i % 26))
			}
			fileName = string(nameBytes)
		}

		data := serializeFileRequest(bm.fileID, fileName, bm.fileSize)

		b.Run(bm.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, _, _, _ = deserializeFileRequest(data)
			}
		})
	}
}

// BenchmarkSerializeFileData benchmarks file data chunk serialization.
func BenchmarkSerializeFileData(b *testing.B) {
	benchmarks := []struct {
		name      string
		fileID    uint32
		chunkSize int
	}{
		{"small_chunk_128B", 1, 128},
		{"default_chunk_1KB", 2, ChunkSize},
		{"large_chunk_64KB", 3, MaxChunkSize},
	}

	for _, bm := range benchmarks {
		chunk := make([]byte, bm.chunkSize)
		// Fill with pattern data
		for i := range chunk {
			chunk[i] = byte(i % 256)
		}

		b.Run(bm.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = serializeFileData(bm.fileID, chunk)
			}
		})
	}
}

// BenchmarkDeserializeFileData benchmarks file data chunk deserialization.
func BenchmarkDeserializeFileData(b *testing.B) {
	benchmarks := []struct {
		name      string
		fileID    uint32
		chunkSize int
	}{
		{"small_chunk_128B", 1, 128},
		{"default_chunk_1KB", 2, ChunkSize},
		{"large_chunk_64KB", 3, MaxChunkSize},
	}

	for _, bm := range benchmarks {
		chunk := make([]byte, bm.chunkSize)
		for i := range chunk {
			chunk[i] = byte(i % 256)
		}
		data := serializeFileData(bm.fileID, chunk)

		b.Run(bm.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, _, _ = deserializeFileData(data)
			}
		})
	}
}

// BenchmarkSerializeFileDataAck benchmarks file data acknowledgment serialization.
func BenchmarkSerializeFileDataAck(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = serializeFileDataAck(uint32(i%100), uint64(i*1024))
	}
}

// BenchmarkSerializeDeserializeRoundTrip benchmarks full serialize-deserialize cycle.
func BenchmarkSerializeDeserializeRoundTrip(b *testing.B) {
	b.Run("file_request", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			data := serializeFileRequest(uint32(i), "document.pdf", 1048576)
			_, _, _, _ = deserializeFileRequest(data)
		}
	})

	b.Run("file_data", func(b *testing.B) {
		chunk := make([]byte, ChunkSize)
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			data := serializeFileData(uint32(i), chunk)
			_, _, _ = deserializeFileData(data)
		}
	})
}

// BenchmarkValidatePath benchmarks path validation for security checks.
func BenchmarkValidatePath(b *testing.B) {
	benchmarks := []struct {
		name string
		path string
	}{
		{"simple_file", "test.txt"},
		{"nested_path", "uploads/2024/01/document.pdf"},
		{"absolute_path", "/var/data/files/archive.zip"},
		{"with_traversal_attempt", "../../../etc/passwd"},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, _ = ValidatePath(bm.path)
			}
		})
	}
}

// BenchmarkTransferProgressCalculation benchmarks progress percentage calculation.
func BenchmarkTransferProgressCalculation(b *testing.B) {
	transfer := NewTransfer(1, 1, "test.txt", testFileSize1GB, TransferDirectionIncoming)
	transfer.Transferred = 536870912 // 50%

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = transfer.GetProgress()
	}
}

// BenchmarkTransferSpeedCalculation benchmarks speed estimation.
func BenchmarkTransferSpeedCalculation(b *testing.B) {
	transfer := NewTransfer(1, 1, "test.txt", testFileSize1GB, TransferDirectionIncoming)
	transfer.State = TransferStateRunning
	transfer.transferSpeed = 1048576 // 1 MB/s

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = transfer.GetSpeed()
	}
}

// BenchmarkTransferTimeRemaining benchmarks ETA calculation.
func BenchmarkTransferTimeRemaining(b *testing.B) {
	transfer := NewTransfer(1, 1, "test.txt", testFileSize1GB, TransferDirectionIncoming)
	transfer.State = TransferStateRunning
	transfer.Transferred = 536870912 // 50%
	transfer.transferSpeed = 1048576 // 1 MB/s

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = transfer.GetEstimatedTimeRemaining()
	}
}

// BenchmarkIsStalled benchmarks stall detection check.
func BenchmarkIsStalled(b *testing.B) {
	transfer := NewTransfer(1, 1, "test.txt", testFileSize1GB, TransferDirectionIncoming)
	transfer.State = TransferStateRunning
	transfer.stallTimeout = DefaultStallTimeout

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = transfer.IsStalled()
	}
}

// BenchmarkNewTransfer benchmarks transfer creation.
func BenchmarkNewTransfer(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = NewTransfer(uint32(i%256), uint32(i), "benchmark_file.dat", testFileSize1GB, TransferDirectionOutgoing)
	}
}

// BenchmarkTransferKeyLookup benchmarks the manager's transfer lookup performance.
func BenchmarkTransferKeyLookup(b *testing.B) {
	manager := NewManager(nil)

	// Pre-populate with transfers
	for i := uint32(0); i < 1000; i++ {
		manager.transfers[transferKey{friendID: i, fileID: i}] = &Transfer{
			FriendID: i,
			FileID:   i,
		}
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = manager.GetTransfer(uint32(i%1000), uint32(i%1000))
	}
}
