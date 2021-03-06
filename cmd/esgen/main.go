package main

import (
	"encoding/json"
	"flag"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

// property represents property for each field.
type property struct {
	Type       string
	Length     int
	Prefix     string
	Value      interface{}
	Multi      bool
	Max        float64
	Properties map[string]*property
	Num        int
	List       []interface{}
	Seed       string
}

// gen generates and returns value of the property.
func (p *property) gen(seq int, seeds map[string]interface{}) interface{} {
	if p.Properties != nil {
		s := make([]interface{}, p.Num)

		for i := range s {
			s[i] = genProps(p.Properties, seq)
		}

		return s
	}

	switch p.Value {
	case "$seq":
		var s string

		if p.Max == 0 {
			s = strconv.Itoa(seq)
		} else {
			if seq%int(p.Max) == 0 {
				s = strconv.Itoa(int(p.Max))
			} else {
				s = strconv.Itoa(seq % int(p.Max))
			}
		}

		if p.Length == 0 {
			return p.Prefix + s
		}

		return p.Prefix + strings.Repeat("0", p.Length-len(p.Prefix)-len(s)) + s
	case "$seq_int":
		return seq
	case "$rand_num":
		if p.Multi {
			s := make([]string, rand.Intn(5)+1)

			for i := range s {
				s[i] = randNum(p.Length)
			}

			return s
		}

		return randNum(p.Length)
	case "$rand_int":
		return randInt(int(p.Max))
	case "$rand_double":
		return randDouble(p.Max)
	case "$rand_kana":
		if p.Multi {
			s := make([]string, rand.Intn(5)+1)

			for i := range s {
				s[i] = randKana(p.Length / 2)
			}

			return s
		}

		return randKana(p.Length / 2)
	case "$rand_bool":
		return randBool()
	case "$rand_date":
		return randDate()
	case "$rand_list":
		if p.Multi {
			s := make([]interface{}, rand.Intn(5)+1)

			for i := range s {
				s[i] = p.List[rand.Intn(len(p.List))]
			}

			return s
		}

		return p.List[rand.Intn(len(p.List))]
	case "$seed_list":
		seedInt, err := strconv.Atoi(seeds[p.Seed].(string))
		if err != nil {
			panic(err)
		}
		return p.List[(seedInt-1)%len(p.List)]
	default:
		return p.Value
	}
}

// config represents configuration for the processing.
type config struct {
	Action        string
	Index         string
	Type          string
	Num           int
	Seeds         map[string]*property
	Props         map[string]*property
	MaxNumPerFile int `json:"max_num_per_file"`
}

// Flags
var (
	inPath  = flag.String("i", "", "input file path")
	outPath = flag.String("o", "", "output file path")
)

// Kana
var (
	kanas = []string{
		"あ", "い", "う", "え", "お", "か", "き", "く", "け", "こ",
		"さ", "し", "す", "せ", "そ", "た", "ち", "つ", "て", "と",
		"な", "に", "ぬ", "ね", "の", "は", "ひ", "ふ", "へ", "ほ",
		"ま", "み", "む", "め", "も", "や", "ゆ", "よ",
		"ら", "り", "る", "れ", "ろ", "わ", "を", "ん",
		"が", "ぎ", "ぐ", "げ", "ご", "ざ", "じ", "ず", "ぜ", "ぞ",
		"だ", "ぢ", "づ", "で", "ど", "ば", "び", "ぶ", "べ", "ぼ",
		"ぱ", "ぴ", "ぷ", "ぺ", "ぽ",
	}

	kanaLen = len(kanas)
)

// The maximum number of CPUs
var maxprocs int

// Wait group
var wg sync.WaitGroup

// Mutex
var mu sync.Mutex

// LF
var lf = []byte("\n")

// Files
var files = make([]*os.File, 0)

// Config
var conf config

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	maxprocs = runtime.GOMAXPROCS(0)

	flag.Parse()
}

type context struct {
	w  io.Writer
	bs [][]byte
}

func write(ctxc <-chan *context) {
	for ctx := range ctxc {
		mu.Lock()

		for _, b := range ctx.bs {
			if _, err := ctx.w.Write(b); err != nil {
				mu.Unlock()
				panic(err)
			}

			ctx.w.Write(lf)
		}

		mu.Unlock()
	}

	wg.Done()
}

func main() {
	in, err := ioutil.ReadFile(*inPath)
	if err != nil {
		panic(err)
	}

	if err := json.Unmarshal(in, &conf); err != nil {
		panic(err)
	}

	ctxc := make(chan *context)

	for i := 0; i < maxprocs; i++ {
		wg.Add(1)
		go write(ctxc)
	}

	defer func() {
		close(ctxc)

		wg.Wait()

		for _, f := range files {
			f.Close()
		}
	}()

	var f *os.File

	var numPerFile int
	var seqFile int

	for seq := 1; seq <= conf.Num; seq++ {
		numPerFile++

		if seq == 1 || numPerFile > conf.MaxNumPerFile {
			seqFile++

			strSeq := strconv.Itoa(seqFile)

			f, err = os.Create(*outPath + "." + strings.Repeat("0", 4-len(strSeq)) + strSeq)
			if err != nil {
				panic(err)
			}

			files = append(files, f)

			numPerFile = 1
		}

		meta := make(map[string]string)

		meta["_index"] = conf.Index
		meta["_type"] = conf.Type
		meta["_id"] = conf.Props["_id"].gen(seq, nil).(string)

		if p, exist := conf.Props["_parent"]; exist {
			meta["_parent"] = p.gen(seq, nil).(string)
		}

		action := map[string]map[string]string{
			conf.Action: meta,
		}

		actionOut, err := json.Marshal(action)
		if err != nil {
			panic(err)
		}

		srcOut, err := json.Marshal(genProps(conf.Props, seq))
		if err != nil {
			panic(err)
		}

		ctxc <- &context{
			w:  f,
			bs: [][]byte{actionOut, srcOut},
		}
	}
}

func genProps(props map[string]*property, seq int) map[string]interface{} {
	// Seeds
	var seeds = make(map[string]interface{})

	for k, p := range conf.Seeds {
		seeds[k] = p.gen(seq, nil)
	}

	src := make(map[string]interface{})

	for k, p := range props {
		if k == "_id" || k == "_parent" {
			continue
		}

		src[k] = p.gen(seq, seeds)
	}

	return src
}

// randNum generates and returns a random number.
func randNum(l int) string {
	var s string

	for i := 0; i < l; i++ {
		s += strconv.Itoa(rand.Intn(10))
	}

	return s
}

// randInt generates and returns a random integer value.
func randInt(n int) int {
	return rand.Intn(n)
}

// randDouble generates and returns a random double value.
func randDouble(n float64) float64 {
	return rand.Float64() * n
}

// randKana generates and returns a random kana.
func randKana(l int) string {
	var s string

	for i := 0; i < l; i++ {
		s += kanas[rand.Intn(kanaLen)]
	}

	return s
}

// randBool generates and returns a random boolean value.
func randBool() bool {
	return rand.Intn(2) == 1
}

// randDate generates and returns a random date value.
func randDate() string {
	m := strconv.Itoa(rand.Intn(3) + 1)

	if len(m) < 2 {
		m = "0" + m
	}

	var maxD int

	switch m {
	case "01", "03", "05", "07", "08", "10", "12":
		maxD = 31
	case "02":
		maxD = 28
	default:
		maxD = 30
	}

	d := strconv.Itoa(rand.Intn(maxD) + 1)

	if len(d) < 2 {
		d = "0" + d
	}

	return "2015" + m + d
}
