package zipscanner

import (
	"archive/zip"
	"bufio"
	"compress/flate"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
)

// from archive/zip struct.go

const (
	fileHeaderSignature      = 0x04034b50
	directoryHeaderSignature = 0x02014b50
	dataDescriptorSignature  = 0x08074b50 // de-facto standard; required by OS X Finder
	fileHeaderLen            = 30         // + filename + extra
	dataDescriptorLen        = 16         // four uint32: descriptor signature, crc32, compressed size, size
	dataDescriptor64Len      = 24         // descriptor with 8 byte sizes

	// version numbers
	zipVersion20 = 20 // 2.0
	zipVersion45 = 45 // 4.5 (reads and writes zip64 archives)

	hasDataDescriptor = 0x08 // mask for records with extra data descriptor
)

var (
	InvalidFileHeader           = errors.New("invalid file header")
	InvalidDataDescriptorHeader = errors.New("invalid data descriptor header")
	UnsupportedCompression      = errors.New("unsupported compression mode")
	NoUncompressedSize          = errors.New("missing uncompressed size")
)

type readBuf []byte

func (b *readBuf) uint16() uint16 {
	v := binary.LittleEndian.Uint16(*b)
	*b = (*b)[2:]
	return v
}

func (b *readBuf) uint32() uint32 {
	v := binary.LittleEndian.Uint32(*b)
	*b = (*b)[4:]
	return v
}

func (b *readBuf) uint64() uint64 {
	v := binary.LittleEndian.Uint64(*b)
	*b = (*b)[8:]
	return v
}

type ZipScannerImpl struct {
	Debug bool

	reader *bufio.Reader

	done bool // done extracting

	fh  zip.FileHeader // last file info
	fr  io.Reader      // last file reader
	err error          // last error
}

func NewZipScanner(r io.Reader) *ZipScannerImpl {
	zr := ZipScannerImpl{}

	if br, ok := r.(*bufio.Reader); ok {
		zr.reader = br
	} else {
		zr.reader = bufio.NewReader(r)
	}

	return &zr
}

func (r *ZipScannerImpl) Error() error {
	return r.err
}

func (r *ZipScannerImpl) FileHeader() zip.FileHeader {
	return r.fh
}

func (r *ZipScannerImpl) stop(done bool, err error) bool {
	if r.Debug {
		fmt.Println("stop", done, err)
	}

	r.done = done
	r.err = err

	return !r.done
}

func (r *ZipScannerImpl) readError(err error) (io.Reader, error) {
	if r.Debug {
		fmt.Println("readError", err)
	}

	r.done = true
	r.err = err
	return nil, r.err
}

func (r *ZipScannerImpl) Scan() bool {
	if r.fr != nil {
		if !r.done && (r.fh.Flags&hasDataDescriptor) != 0 {
			// data descriptor
			var dd [dataDescriptorLen]byte

			if _, err := io.ReadFull(r.reader, dd[:]); err != nil {
				return r.stop(true, err)
			}

			if r.Debug {
				fmt.Println(hex.Dump(dd[:]))
			}

			b := readBuf(dd[:])
			magic := b.uint32()
			crc := b.uint32()
			csize := b.uint32()
			usize := b.uint32()

			if r.Debug {
				fmt.Println()
				fmt.Printf("magic   %08x\n", magic)
				fmt.Printf("crc32   %08x\n", crc)
				fmt.Printf("compressed size   %d\n", csize)
				fmt.Printf("uncompressed size %d\n", usize)
			}

			if magic != dataDescriptorSignature {
				return r.stop(true, InvalidDataDescriptorHeader)
			}
		}

		if rc, ok := r.fr.(io.ReadCloser); ok {
			rc.Close()
		}
		r.fr = nil
	}

	if r.done {
		if r.Debug {
			fmt.Println("Done")
		}

		return false
	}

	var fh [fileHeaderLen]byte

	if _, err := io.ReadFull(r.reader, fh[:]); err != nil {
		return r.stop(true, err)
	}

	if r.Debug {
		fmt.Println(hex.Dump(fh[:]))
	}

	b := readBuf(fh[:])
	magic := b.uint32()

	if magic == directoryHeaderSignature {
		// got central directory. Done
		return r.stop(true, io.EOF)
	}

	if magic != fileHeaderSignature {
		return r.stop(true, InvalidFileHeader)
	}

	r.fh.CreatorVersion = b.uint16()
	r.fh.Flags = b.uint16()
	r.fh.Method = b.uint16()
	r.fh.ModifiedTime = b.uint16()
	r.fh.ModifiedDate = b.uint16()
	r.fh.CRC32 = b.uint32()
	r.fh.CompressedSize = b.uint32()
	r.fh.UncompressedSize = b.uint32()
	r.fh.CompressedSize64 = uint64(r.fh.CompressedSize)
	r.fh.UncompressedSize64 = uint64(r.fh.UncompressedSize)

	flen := b.uint16()
	elen := b.uint16()

	d := make([]byte, flen+elen)
	if _, err := io.ReadFull(r.reader, d); err != nil {
		return r.stop(true, err)
	}

	r.fh.Name = string(d[:flen])
	r.fh.Extra = d[flen : flen+elen]

	return r.stop(false, nil)
}

func (r *ZipScannerImpl) Reader() (io.Reader, error) {
	switch r.fh.Method {
	case zip.Deflate:
		if r.Debug {
			fmt.Println("inflating...")
		}

		r.fr = flate.NewReader(r.reader)

	case zip.Store:
		if r.Debug {
			fmt.Println("reading...")
		}

		if r.fh.UncompressedSize > 0 {
			r.fr = io.LimitReader(r.reader, int64(r.fh.UncompressedSize))
		} else if r.fh.UncompressedSize == 0 && (r.fh.Flags&hasDataDescriptor) == 0 {
			// file of 0 bytes or directory ?
			r.fr = io.LimitReader(r.reader, 0)
		} else {
			return r.readError(NoUncompressedSize)
		}

	default:
		return r.readError(UnsupportedCompression)
	}

	r.err = nil
	return r.fr, r.err
}
