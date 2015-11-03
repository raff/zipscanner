package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
	"strings"

	"github.com/raff/zipscanner"
	"github.com/gobs/httpclient"
)

func main() {
	debug := flag.Bool("debug", false, "print debug info")
	//view := flag.Bool("v", false, "view list")
	//out := flag.String("out", "", "write recovered files to output zip file")
	//override := flag.Bool("override", false, "override existing files")

	flag.Parse()

	if flag.NArg() != 1 {
		log.Fatal("usage: ", path.Base(os.Args[0]), " {zip-file}")
	}

	zipfile := flag.Arg(0)

	var reader io.Reader

	if strings.Contains(zipfile, "://") { // URL
		resp, err := httpclient.NewHttpClient(zipfile).Get("", nil, nil)
		if err != nil {
			log.Println(zipfile, err)
			return
		}

		defer resp.Close()
		reader = resp.Body
	} else {
		f, err := os.Open(zipfile)
		if err != nil {
			log.Fatal("open ", err)
		}

		defer f.Close()
		reader = f
	}

	zs := zipscanner.NewZipScanner(reader)
	zs.Debug = *debug

	count := 0

	for zs.Scan() {
		f := zs.FileHeader()
		fmt.Printf("%8d %8d %8x %s\n", f.CompressedSize, f.UncompressedSize, f.CRC32, f.Name)

		r, err := zs.Reader()
		if err != nil {
			log.Fatal(err)
		}

		io.Copy(ioutil.Discard, r)

		count += 1
	}

	fmt.Println("total", count)

	if zs.Error() != io.EOF {
		log.Fatal(zs.Error())
	}
}
