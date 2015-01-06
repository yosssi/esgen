package main

import (
	"flag"
	"fmt"
	"io/ioutil"
)

// Flags
var (
	inPath  = flag.String("i", "", "input file path")
	outPath = flag.String("o", "", "output file path")
)

func init() {
	flag.Parse()
}

func main() {
	in, err := ioutil.ReadFile(*inPath)
	if err != nil {
		panic(err)
	}

	fmt.Println(string(in))
}
