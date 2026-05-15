package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"time"

	"heimdall/internal/reference"
)

func main() {
	rbinPath := flag.String("rbin", "", "entrada: references.rbin")
	outPath := flag.String("out", "", "saída: references.ivf")
	nList := flag.Int("lists", 512, "número de listas (clusters) IVF; 2048 em 3M vetores demora horas")
	maxIter := flag.Int("iter", 12, "máximo de iterações k-means")
	seed := flag.Int64("seed", 42, "seed para clusters vazios / tie-break")
	workers := flag.Int("workers", 0, "goroutines de atribuição (0 = todos os CPUs)")
	flag.Parse()
	if *rbinPath == "" || *outPath == "" {
		flag.Usage()
		os.Exit(2)
	}
	if *nList < 1 {
		log.Fatal("lists deve ser >= 1")
	}
	if *workers == 0 {
		*workers = runtime.NumCPU()
	}

	log.Printf("genivf: lists=%d iter=%d workers=%d", *nList, *maxIter, *workers)
	log.Printf("genivf: com 3M vetores, lists=512 leva ~5-15 min; lists=2048 pode levar 1h+")

	start := time.Now()
	n, nLO, cents, offs, posts, err := reference.TrainIVFFromRBinConfig(*rbinPath, reference.TrainIVFConfig{
		NList:   *nList,
		MaxIter: *maxIter,
		Seed:    *seed,
		Workers: *workers,
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("genivf: a gravar %s ...", *outPath)
	if err := reference.WriteIVFFile(*outPath, n, nLO, cents, offs, posts); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("gravado %s (n=%d lists=%d) em %s\n", *outPath, n, nLO, time.Since(start).Round(time.Second))
}
