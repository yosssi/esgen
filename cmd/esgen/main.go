package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"os"
)

// property represents property for each field.
type property struct {
	Type   string
	Length int
	Prefix string
	Value  string
}

// config represents configuration for the processing.
type config struct {
	Action string
	Index  string
	Type   string
	Num    int
	Props  map[string]property
}

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

	var conf config
	if err := json.Unmarshal(in, &conf); err != nil {
		panic(err)
	}

	f, err := os.Create(*outPath)
	if err != nil {
		panic(err)
	}

	defer f.Close()

	for i := 0; i < conf.Num; i++ {
		meta := make(map[string]string)

		meta["_index"] = conf.Index
		meta["_type"] = conf.Type
		meta["_id"] = conf.Props["_id"].Value

		action := map[string]map[string]string{
			conf.Action: meta,
		}

		out, err := json.Marshal(action)
		if err != nil {
			panic(err)
		}

		if _, err := f.Write(out); err != nil {
			panic(err)
		}

		f.WriteString("\n")
	}
}
