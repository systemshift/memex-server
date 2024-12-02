package common

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"sync"
)

// Magic numbers for different sections
const (
	FileMagic  uint64 = 0x4D454D4558464C45 // "MEMEXFLE"
	ChunkMagic uint32 = 0x43484B           // "CHK"
	NodeMagic  uint32 = 0x4E4F44           // "NOD"
	LinkMagic  uint32 = 0x4C4E4B           // "LNK"
	TxMagic    uint32 = 0x54584E           // "TXN"
)

// File represents a storage file with thread-safe operations
type File struct {
	file     *os.File
	mutex    sync.RWMutex
	position int64
}

// OpenFile opens a file with the given path and flags
func OpenFile(path string, flag int, perm os.FileMode) (*File, error) {
	f, err := os.OpenFile(path, flag, perm)
	if err != nil {
		return nil, fmt.Errorf("opening file: %w", err)
	}

	return &File{
		file: f,
	}, nil
}

// Name returns the name of the underlying file
func (f *File) Name() string {
	return f.file.Name()
}

// Read implements io.Reader
func (f *File) Read(p []byte) (n int, err error) {
	f.mutex.RLock()
	defer f.mutex.RUnlock()

	n, err = f.file.Read(p)
	if err == nil {
		f.position += int64(n)
	}
	return n, err
}

// Write implements io.Writer
func (f *File) Write(p []byte) (n int, err error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	n, err = f.file.Write(p)
	if err == nil {
		f.position += int64(n)
	}
	return n, err
}

// Close closes the file
func (f *File) Close() error {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if f.file == nil {
		return nil
	}

	err := f.file.Close()
	f.file = nil
	return err
}

// ReadAt reads len(b) bytes from the file starting at byte offset off
func (f *File) ReadAt(b []byte, off int64) (n int, err error) {
	f.mutex.RLock()
	defer f.mutex.RUnlock()

	return f.file.ReadAt(b, off)
}

// WriteAt writes len(b) bytes to the file starting at byte offset off
func (f *File) WriteAt(b []byte, off int64) (n int, err error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	return f.file.WriteAt(b, off)
}

// Seek sets the offset for the next Read or Write on file to offset
func (f *File) Seek(offset int64, whence int) (int64, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	pos, err := f.file.Seek(offset, whence)
	if err != nil {
		return 0, err
	}

	f.position = pos
	return pos, nil
}

// Sync commits the current contents of the file to stable storage
func (f *File) Sync() error {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	return f.file.Sync()
}

// ReadUint32 reads a uint32 from the current position
func (f *File) ReadUint32() (uint32, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	var val uint32
	if err := binary.Read(f.file, binary.LittleEndian, &val); err != nil {
		return 0, err
	}
	f.position += 4
	return val, nil
}

// WriteUint32 writes a uint32 at the current position
func (f *File) WriteUint32(val uint32) error {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if err := binary.Write(f.file, binary.LittleEndian, val); err != nil {
		return err
	}
	f.position += 4
	return nil
}

// ReadUint64 reads a uint64 from the current position
func (f *File) ReadUint64() (uint64, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	var val uint64
	if err := binary.Read(f.file, binary.LittleEndian, &val); err != nil {
		return 0, err
	}
	f.position += 8
	return val, nil
}

// WriteUint64 writes a uint64 at the current position
func (f *File) WriteUint64(val uint64) error {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if err := binary.Write(f.file, binary.LittleEndian, val); err != nil {
		return err
	}
	f.position += 8
	return nil
}

// ReadBytes reads n bytes from the current position
func (f *File) ReadBytes(n int) ([]byte, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	b := make([]byte, n)
	if _, err := io.ReadFull(f.file, b); err != nil {
		return nil, err
	}
	f.position += int64(n)
	return b, nil
}

// WriteBytes writes bytes at the current position
func (f *File) WriteBytes(b []byte) error {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	n, err := f.file.Write(b)
	if err != nil {
		return err
	}
	f.position += int64(n)
	return nil
}

// CalculateChecksum calculates CRC32 checksum of data
func CalculateChecksum(data []byte) uint32 {
	return crc32.ChecksumIEEE(data)
}

// ValidateChecksum validates data against its checksum
func ValidateChecksum(data []byte, checksum uint32) bool {
	return CalculateChecksum(data) == checksum
}
