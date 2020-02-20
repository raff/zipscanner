package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/gobs/httpclient"
	"github.com/raff/zipscanner"
)

func main() {
	debug := flag.Bool("debug", false, "print debug info")
	extract := flag.Bool("extract", false, "extract files")
	nodir := flag.Bool("no-dir", false, "don't create subdirectories - extract in current directory")
	match := flag.String("match", "", "match filenames to extract")

	flag.Parse()

	if flag.NArg() != 1 {
		log.Fatal("usage: ", path.Base(os.Args[0]), " {zip-file}")
	}

	zipfile := flag.Arg(0)

	var match_re *regexp.Regexp

	if *match != "" {
		match_re = regexp.MustCompile(*match)
	}

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

		process := match_re == nil || match_re.MatchString(f.Name)

		if process {
			fmt.Printf("%8d %8d %8x %s\n", f.CompressedSize, f.UncompressedSize, f.CRC32, f.Name)
			count += 1
		}

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

			if process {
				filename := f.Name

				if *nodir {
					filename = path.Base(filename)
				} else {
					dir := path.Dir(filename)
					if dir != "" && dir != lastdir {
						err = os.Mkdir(dir, os.ModeDir|0755)
						if err != nil {
							log.Println(err)
						} else {
							lastdir = dir
						}
					}
				}

				fw, err = os.Create(filename)
				if err != nil {
					log.Println(err)
				} else {
					w = fw
				}
			}
		}

		io.Copy(w, r)

		if fw != nil {
			fw.Close()
		}
	}

	fmt.Println("total", count)

	if zs.Error() != io.EOF {
		log.Fatal(zs.Error())
	}
}
