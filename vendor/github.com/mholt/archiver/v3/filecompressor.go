package archiver

import (
	"fmt"
	"os"
)

// FileCompressor can compress and decompress single files.
type FileCompressor struct {
	Compressor
	Decompressor

	// Whether to overwrite existing files when creating files.
	OverwriteExisting bool
}

// CompressFile reads the source file and compresses it to destination.
// The destination must have a matching extension.
func (fc FileCompressor) CompressFile(source, destination string) error {
	if err := fc.CheckExt(destination); err != nil {
		return err
	}
	if fc.Compressor == nil {
		return fmt.Errorf("no compressor specified")
	}
	if !fc.OverwriteExisting && fileExists(destination) {
		return fmt.Errorf("file exists: %s", destination)
	}

	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(destination)
	if err != nil {
		return err
	}
	defer out.Close()

	return fc.Compress(in, out)
}

// DecompressFile reads the source file and decompresses it to destination.
func (fc FileCompressor) DecompressFile(source, destination string) error {
	if fc.Decompressor == nil {
		return fmt.Errorf("no decompressor specified")
	}
	if !fc.OverwriteExisting && fileExists(destination) {
		return fmt.Errorf("file exists: %s", destination)
	}

	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(destination)
	if err != nil {
		return err
	}
	defer out.Close()

	return fc.Decompress(in, out)
}
