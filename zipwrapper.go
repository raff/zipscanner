package zipscanner

import (
	"io"
	"archive/zip"
	)

//
// ZipWrapper wraps a zip.Reader into a ZipScanner compatible interface
//
type ZipWrapper struct {
	zr  *zip.Reader
	p   int
	fr  io.ReadCloser
	err error
}

func NewZipWrapper(z *zip.Reader) *ZipWrapper {
	return &ZipWrapper{zr: z, p: -1}
}

func (z *ZipWrapper) Scan() bool {
	if z.fr != nil {
		z.fr.Close()
		z.fr = nil
	}

	z.p += 1

	if z.p >= len(z.zr.File) {
		z.err = io.EOF
		return false
	}

	z.err = nil
	return true
}

func (z *ZipWrapper) FileHeader() zip.FileHeader {
	return z.zr.File[z.p].FileHeader
}

func (z *ZipWrapper) Reader() (io.Reader, error) {
	if r, err := z.zr.File[z.p].Open(); err != nil {
		z.err = err
		return nil, err
	} else {
		z.fr = r
		return r, nil
	}
}

func (z *ZipWrapper) Error() error {
	return z.err
}

