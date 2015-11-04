package zipscanner

import "io"
import "archive/zip"

type ZipScanner interface {
	Scan() bool
	FileHeader() zip.FileHeader
	Reader() (io.Reader, error)
	Error() error
}
