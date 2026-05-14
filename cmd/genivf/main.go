package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"heimdall/internal/reference"
)

func main() {
	rbinPath := flag.String("rbin", "", "entrada: references.rbin")
	outPath := flag.String("out", "", "saída: references.ivf")
	nList := flag.Int("lists", 512, "número de listas (clusters) IVF")
	maxIter := flag.Int("iter", 20, "máximo de iterações k-means")
	seed := flag.Int64("seed", 42, "seed para clusters vazios / tie-break")
	flag.Parse()
	if *rbinPath == "" || *outPath == "" {
		flag.Usage()
		os.Exit(2)
	}
	if *nList < 1 {
		log.Fatal("lists deve ser >= 1")
	}

	n, nLO, cents, offs, posts, err := reference.TrainIVFFromRBin(*rbinPath, *nList, *maxIter, *seed)
	if err != nil {
		log.Fatal(err)
	}
	if err := reference.WriteIVFFile(*outPath, n, nLO, cents, offs, posts); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("gravado %s (n=%d lists=%d)\n", *outPath, n, nLO)
}
