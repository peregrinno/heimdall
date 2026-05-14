package main

import (
	"compress/gzip"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"heimdall/internal/reference"
)

func main() {
	inPath := flag.String("in", "", "entrada: references.json.gz ou .json")
	outPath := flag.String("out", "", "saída: references.rbin")
	flag.Parse()
	if *inPath == "" || *outPath == "" {
		flag.Usage()
		os.Exit(2)
	}

	f, err := os.Open(*inPath)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	var r io.Reader = f
	if strings.HasSuffix(strings.ToLower(*inPath), ".gz") {
		gz, err := gzip.NewReader(f)
		if err != nil {
			log.Fatal(err)
		}
		defer func() { _ = gz.Close() }()
		r = gz
	}

	n, err := reference.BuildRBin(r, *outPath)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("gravado %s (%d vetores)\n", *outPath, n)
}
