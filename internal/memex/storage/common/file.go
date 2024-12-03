package common

import (
	"encoding/binary"
	"fmt"
	"os"
)

// File represents a file with additional utilities
type File struct {
	*os.File
}

// OpenFile opens a file with the given flags and permissions
func OpenFile(path string, flag int, perm os.FileMode) (*File, error) {
	file, err := os.OpenFile(path, flag, perm)
	if err != nil {
		return nil, err
	}
	return &File{file}, nil
}

// ReadUint32 reads a uint32 in little endian
func (f *File) ReadUint32() (uint32, error) {
	var value uint32
	if err := binary.Read(f, binary.LittleEndian, &value); err != nil {
		return 0, fmt.Errorf("reading uint32: %w", err)
	}
	return value, nil
}

// WriteUint32 writes a uint32 in little endian
func (f *File) WriteUint32(value uint32) error {
	if err := binary.Write(f, binary.LittleEndian, value); err != nil {
		return fmt.Errorf("writing uint32: %w", err)
	}
	return nil
}

// ReadUint64 reads a uint64 in little endian
func (f *File) ReadUint64() (uint64, error) {
	var value uint64
	if err := binary.Read(f, binary.LittleEndian, &value); err != nil {
		return 0, fmt.Errorf("reading uint64: %w", err)
	}
	return value, nil
}

// WriteUint64 writes a uint64 in little endian
func (f *File) WriteUint64(value uint64) error {
	if err := binary.Write(f, binary.LittleEndian, value); err != nil {
		return fmt.Errorf("writing uint64: %w", err)
	}
	return nil
}

// ReadExactly reads exactly size bytes from the file
func (f *File) ReadExactly(size int) ([]byte, error) {
	buf := make([]byte, size)
	n, err := f.Read(buf)
	if err != nil {
		return nil, err
	}
	if n != size {
		return nil, fmt.Errorf("short read: got %d bytes, want %d", n, size)
	}
	return buf, nil
}

// WriteExactly writes all bytes to the file
func (f *File) WriteExactly(data []byte) error {
	n, err := f.Write(data)
	if err != nil {
		return err
	}
	if n != len(data) {
		return fmt.Errorf("short write: wrote %d bytes, want %d", n, len(data))
	}
	return nil
}

// Sync commits the current contents of the file to stable storage.
func (f *File) Sync() error {
	return f.File.Sync()
}

// Close closes the file.
func (f *File) Close() error {
	return f.File.Close()
}
