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

	"github.com/gobs/httpclient"
	"github.com/raff/zipscanner"
)

func main() {
	debug := flag.Bool("debug", false, "print debug info")
	//view := flag.Bool("v", false, "view list")
	//out := flag.String("out", "", "write recovered files to output zip file")
	//override := flag.Bool("override", false, "override existing files")
	extract := flag.Bool("extract", false, "extract files")

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
	lastdir := ""

	for zs.Scan() {
		f := zs.FileHeader()
		fmt.Printf("%8d %8d %8x %s\n", f.CompressedSize, f.UncompressedSize, f.CRC32, f.Name)

		r, err := zs.Reader()
		if err != nil {
			log.Fatal(err)
		}

		w := ioutil.Discard

		var fw *os.File

		if *extract {
			var err error

			if strings.HasSuffix(f.Name, "/") { // assume is a folder
				if f.UncompressedSize != 0 {
					log.Println("folder", f.Name, "size", f.UncompressedSize)
				}

				continue
			}

			dir := path.Dir(f.Name)
			if dir != "" && dir != lastdir {
				err = os.Mkdir(dir, os.ModeDir|0755)
				if err != nil {
					log.Println(err)
				} else {
					lastdir = dir
				}
			}

			fw, err = os.Create(f.Name)
			if err != nil {
				log.Println(err)
			} else {
				w = fw
			}
		}

		io.Copy(w, r)

		if fw != nil {
			fw.Close()
		}

		count += 1
	}

	fmt.Println("total", count)

	if zs.Error() != io.EOF {
		log.Fatal(zs.Error())
	}
}
