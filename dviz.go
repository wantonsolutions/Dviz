package main

import (
	//	"encoding/gob"
	"encoding/json"
	"flag"
	"io"
	"net/http"
	"os"
	"runtime"
	"runtime/pprof"

	logging "github.com/op/go-logging"
	//"encoding/gob"
	"bytes"
	"fmt"
	"math"
	"math/big"
	"os/exec"
	"regexp"

	"bitbucket.org/bestchai/dinv/logmerger"
)

const (
	regex  = `(?P<Host>[[:alpha:]]*)-(?P<Package>[[:alpha:]]*)_(?P<File>[[:alpha:]]*)_(?P<Line>[[:digit:]]*)_(?P<Name>[[:alnum:]]*)`
	regex2 = `(?P<Host>[[:alnum:]]*)-(?P<Name>.*)`
)

var (
	logger = logging.MustGetLogger("Dviz")
	format = logging.MustStringFormatter(
		`%{color}%{time:15:04:05.000} %{shortfunc} ▶ %{level:.4s} %{id:03x}%{color:reset} %{message}`,
	)
	serverMode = flag.Bool("s", false, "launch in server mode Port 3119")
	port       = flag.String("p", "3119", "server listening port")
	loglevel   = flag.Int("ll", 0, "log level 5:Debug 4:Info 3:Notice 2:Warning 1:Error 0:Critical")
	fast       = flag.Bool("fast", false, "fast drops general structures like maps for the sake of speed")
	filename   = flag.String("file", "", "filename: file must be json dinv output")
	outputfile = flag.String("output", "output.json", "output filename: filename to output to")
	logfile    = flag.String("log", "", "logfile: log to file")
	cpuprofile = flag.String("cpuprofile", "", "write cpu profile `file`")
	memprofile = flag.String("memprofile", "", "write memory profile to `file`")

	difference   func(a, b interface{}) int64
	draw         = false
	render       string
	total        int
	targetMisses int
	badTesting   = false
)

type StatePlane struct {
	States []logmerger.State
	Plane  [][]float64
}

func handler(w http.ResponseWriter, r *http.Request) {
	logger.Infof("Received Request from %s", r.Host)
	states := decodeAndCorrect(r.Body)
	dplane := dviz(states)
	//buf, err := ioutil.ReadFile("." + r.URL.Path + "/index.html")
	enc := json.NewEncoder(w)
	enc.Encode(dplane)
}

func main() {
	flag.Parse()
	//set difference function
	difference = xor
	setupLogger()
	//setupProfiler()
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			logger.Fatal("could not create CPU profile: ", err)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			logger.Fatal("could not start CPU profile: ", err)
		}
		defer pprof.StopCPUProfile()
	}

	if badTesting {
		runBadTests()
		os.Exit(1)
	}

	if *serverMode {
		logger.Notice("Starting Dviz Server!")
		logger.Fatal(http.ListenAndServe(":"+*port, http.HandlerFunc(handler)))
	} else if *filename != "" {
		executeFile()
	}

	if *memprofile != "" {
		f, err := os.Create(*memprofile)
		if err != nil {
			logger.Fatal("could not create memory profile: ", err)
		}
		runtime.GC() // get up-to-date statistics
		if err := pprof.WriteHeapProfile(f); err != nil {
			logger.Fatal("could not write memory profile: ", err)
		}
		f.Close()
	}
}

func executeFile() {
	jsonFile, err := os.Open(*filename)
	if err != nil {
		logger.Fatal(err)
	}
	states := decodeAndCorrect(jsonFile)
	dplane := dviz(states)
	//TODO come up with a proper naming scheme
	output(StatePlane{States: states, Plane: dplane}, *outputfile)
	logger.Debugf("target misses %d\n", targetMisses)
	logger.Debugf("target ration %f\n", float32(targetMisses)/float32(total))

	//plot the single dimension version

	if draw {
		render = "default"
		dat(dplane)
		gnuplotPlane()
		renderImage()
	}

}

func decodeAndCorrect(jsonFile io.ReadCloser) []logmerger.State {
	dec := json.NewDecoder(jsonFile)
	states := make([]logmerger.State, 0)
	var err error
	err = nil
	for err == nil {
		var decodedState logmerger.State
		err = dec.Decode(&decodedState)
		if err != nil && err != io.EOF {
			logger.Fatal(err)
		}
		states = append(states, decodedState)
	}
	if len(states) <= 2 {
		logger.Fatal("error: not enough states to produce a plot\n")
	}
	TypeCorrectJson(&states)
	return states
}

func dviz(states []logmerger.State) [][]float64 {
	vectors := stateVectors(states)
	//dplane := dvizMaster(vectors)
	plane := diff(vectors)
	dplane := mag(plane)
	//fmt.Println(dplane)
	return dplane
}

func output(stateplane StatePlane, outputfile string) {
	outputJson, err := os.Create(outputfile)
	if err != nil {
		logger.Fatal(err)
	}
	enc := json.NewEncoder(outputJson)
	enc.Encode(stateplane)
}

func parseVariables1(name string) (string, string) {
	r := regexp.MustCompile(regex)
	res := r.FindStringSubmatch(name)
	//hardcoded for the
	//machine-package-filename-line-variable parsing
	//logger.Printf("%#v\n", r.FindStringSubmatch(name))
	if len(res) != 6 {
		logger.Fatalf("regex unable to parse variable %s\n", name)
	}
	return res[1], res[2] + res[3] + res[4] + res[5]
}

func parseVariables2(name string) (string, string) {
	r := regexp.MustCompile(regex2)
	res := r.FindStringSubmatch(name)
	//hardcoded for the
	//machine-package-filename-line-variable parsing
	//logger.Printf("%#v\n", r.FindStringSubmatch(name))
	if len(res) != 3 {
		logger.Fatalf("regex unable to parse variable %s\n", name)
	}
	return res[1], res[2]
}

//Json encoding is fast, but it can mess with the types of the
//variables passed to it. For instance integers are converted to
//floating points by adding .00 to them. This function corrects for
//these mistakes and returns the points to their origianl state.
func TypeCorrectJson(states *[]logmerger.State) {
	for i := range *states {
		for j := range (*states)[i].Points {
			for k := range (*states)[i].Points[j].Dump {
				if (*states)[i].Points[j].Dump[k].Type == "int" {
					(*states)[i].Points[j].Dump[k].Value = int((*states)[i].Points[j].Dump[k].Value.(float64))
					// fmt.Printf("type :%s\t value: %s\n",reflect.TypeOf(point.Dump[i].Value).String(),point.Dump[i].value())
				}
			}
		}
	}
}

//TODO I think I broke this the grid should be 300x and it is only 18
func stateVectors(states []logmerger.State) map[string]map[string][]interface{} {
	hostVectors := make(map[string]map[string][]interface{}, 0)
	for i := range states {
		for j := range states[i].Points {
			for k := range states[i].Points[j].Dump {
				host, name := parseVariables2(states[i].Points[j].Dump[k].VarName)
				_, ok := hostVectors[host]
				if !ok {
					hostVectors[host] = make(map[string][]interface{}, len(states[i].Points))
				}
				//logger.Debug(len(states))
				_, ok = hostVectors[host][name]
				if !ok {
					hostVectors[host][name] = make([]interface{}, len(states))
				}
				hostVectors[host][name][i] = states[i].Points[j].Dump[k].Value

			}
		}
	}
	return hostVectors
}

//whole host nxn diff
//returns []stateIndex[]stateIndex[]int64
func diff(vectors map[string]map[string][]interface{}) [][][]int64 {
	//get state array
	var length int
	//get the legnth of the number of states TODO make this better
	for host := range vectors {
		for stubVar := range vectors[host] {
			length = len(vectors[host][stubVar])
			break
		}
		break
	}
	//real algorithm starts here
	plane := make([][][]int64, length)
	for i := 0; i < length; i++ {
		plane[i] = make([][]int64, length)
		for j := 0; j < length; j++ {
			//fmt.Printf("\rCalculating Diff %3.0f%%", 100*(float32(i+1)/float32(len(plane))))
			plane[i][j] = make([]int64, 0)
			for host := range vectors {
				for variable := range vectors[host] {
					plane[i][j] = append(plane[i][j], difference(vectors[host][variable][i], vectors[host][variable][j]))
					total++
				}
			}
		}
	}
	logger.Debugf("Total xor computations: %d\n", total)
	return plane
}

func diff2(vectors map[string]map[string][]interface{}) [][][]int64 {
	//get state array
	var length int
	//get the legnth of the number of states TODO make this better
	for host := range vectors {
		for stubVar := range vectors[host] {
			length = len(vectors[host][stubVar])
			break
		}
		break
	}
	//real algorithm starts here
	plane := make([][][]int64, length)
	for i := 0; i < length; i++ {
		plane[i] = make([][]int64, length)
	}
	for i := 0; i < length; i++ {
		for j := i + 1; j < length; j++ {
			//fmt.Printf("\rCalculating Diff %3.0f%%", 100*(float32(i+1)/float32(len(plane))))
			plane[i][j] = make([]int64, 0)
			plane[j][i] = make([]int64, 0)
			for host := range vectors {
				for variable := range vectors[host] {
					plane[i][j] = append(plane[i][j], difference(vectors[host][variable][i], vectors[host][variable][j]))
					total++
				}
			}
			plane[j][i] = append(plane[j][i], plane[i][j]...)
		}
	}
	logger.Debugf("Total xor computations: %d\n", total)
	return plane
}

func diff3(vectors map[string]map[string][]interface{}) [][][]int64 {
	//get state array
	var length int
	//get the legnth of the number of states TODO make this better
	for host := range vectors {
		for stubVar := range vectors[host] {
			length = len(vectors[host][stubVar])
			break
		}
		break
	}
	//real algorithm starts here
	plane := make([][][]int64, length)
	for i := 0; i < length; i++ {
		plane[i] = make([][]int64, length)
	}
	//launch threads
	input := make(chan Index, 1000)
	output := make(chan Index, 1000)
	for i := 0; i < runtime.NumCPU(); i++ {
		go diffThread(vectors, input, output)
	}
	done := false
	outstanding := 0

	go func() {
		for i := 0; i < length; i++ {
			for j := i + 1; j < length; j++ {
				input <- Index{X: i, Y: j, Diffs: make([]int64, 0)}
				outstanding++
			}
		}
		done = true
	}()

	for !done || outstanding > 0 {
		elem := <-output
		plane[elem.X][elem.Y], plane[elem.Y][elem.X] = elem.Diffs, elem.Diffs
		outstanding--
	}

	logger.Debugf("Total xor computations: %d\n", total)
	return plane
}

type Index struct {
	X     int
	Y     int
	Diffs []int64
}

//diffThread exits when sent an index of -1
func diffThread(vectors map[string]map[string][]interface{}, input chan Index, output chan Index) {
	for true {
		index := <-input
		length := 0
		for host := range vectors {
			length += len(vectors[host])
		}
		index.Diffs = make([]int64, length)
		i := 0
		for host := range vectors {
			for variable := range vectors[host] {
				index.Diffs[i] = difference(vectors[host][variable][index.X], vectors[host][variable][index.Y])
				total++
			}
		}
		output <- index
	}
}

type Index2 struct {
	X    int
	Y    int
	Diff float64
}

func dvizMaster(vectors map[string]map[string][]interface{}) [][]float64 {
	//get state array
	var length int
	//get the legnth of the number of states TODO make this better
	for host := range vectors {
		for stubVar := range vectors[host] {
			length = len(vectors[host][stubVar])
			break
		}
		break
	}
	//real algorithm starts here
	plane := make([][]float64, length)
	for i := 0; i < length; i++ {
		plane[i] = make([]float64, length)
	}
	//launch threads
	input := make(chan Index2, 1000)
	output := make(chan Index2, 1000)
	for i := 0; i < runtime.NumCPU(); i++ {
		go distanceWorker(vectors, input, output)
	}
	done := false
	outstanding := 0

	go func() {
		for i := 0; i < length; i++ {
			for j := i + 1; j < length; j++ {
				input <- Index2{X: i, Y: j, Diff: 0.0}
				outstanding++
			}
		}
		done = true
	}()

	for !done || outstanding > 0 {
		elem := <-output
		plane[elem.X][elem.Y], plane[elem.Y][elem.X] = elem.Diff, elem.Diff
		outstanding--
	}

	logger.Debugf("Total xor computations: %d\n", total)
	return plane

}

func distanceWorker(vectors map[string]map[string][]interface{}, input chan Index2, output chan Index2) {
	for true {
		index := <-input
		var runningDistance int64
		var dVar int64

		for host := range vectors {
			for variable := range vectors[host] {
				dVar = difference(vectors[host][variable][index.X], vectors[host][variable][index.Y])
				runningDistance += dVar * dVar
				total++
			}
		}
		index.Diff = math.Sqrt(float64(runningDistance))
		output <- index
	}
}

func mag(plane [][][]int64) [][]float64 {
	dplane := make([][]float64, len(plane))
	for i := range plane {
		dplane[i] = make([]float64, len(plane))
		for j := range plane[i] {
			var ithMag float64
			for k := range plane[i][j] {
				ithMag += float64(plane[i][j][k]) * float64(plane[i][j][k])
			}
			dplane[i][j] = math.Sqrt(ithMag)
		}
	}
	return dplane
}

func gnuplotPlane() {
	f, err := os.Create(render + ".plot")
	if err != nil {
		logger.Fatal(err)
	}
	f.WriteString("set term png\n")
	f.WriteString("set output \"" + render + ".png\"\n")
	f.WriteString(fmt.Sprintf("plot \"%s.dat\" matrix with image\n", render))

}

func dat(dplane [][]float64) {
	f, err := os.Create(render + ".dat")
	if err != nil {
		logger.Fatal(err)
	}
	for i := range dplane {
		//f.WriteString(fmt.Sprintf("%d\t",i))
		for j := range dplane[i] {
			f.WriteString(fmt.Sprintf("%f\t", dplane[i][j]))
		}
		f.WriteString("\n")
	}
}

func renderImage() {
	cmd := exec.Command("gnuplot", render+".plot")
	if err := cmd.Run(); err != nil {
		logger.Fatal(err)
	}
	cmd = exec.Command("display", render+".png")
	if err := cmd.Run(); err != nil {
		logger.Fatal(err)
	}
}

func xorGeneral(a, b interface{}) int64 {
	//logger.Debugf("xor General on variable %s", a)
	targetMisses++
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	err := enc.Encode(a)
	if err != nil {
		logger.Fatal(err)
	}
	abytes := buf.Bytes()
	buf.Reset()
	err = enc.Encode(b)
	if err != nil {
		logger.Fatal(err)
	}
	bbytes := buf.Bytes()

	//logger.Printf("%s %s\n",abytes,bbytes)
	var xorDiff int64
	var i int
	for i = 0; i < len(abytes) && i < len(bbytes); i++ {
		abig := big.NewInt(int64(abytes[i]))
		bbig := big.NewInt(int64(bbytes[i]))
		z := big.NewInt(0)
		z.Xor(abig, bbig)
		for _, bits := range z.Bits() {
			xorDiff += int64(bitCounts[bits])
		}
	}
	for ; i < len(abytes); i++ {
		xorDiff += 8
	}
	for ; i < len(bbytes); i++ {
		xorDiff += 8
	}
	return xorDiff
}

func xor(a, b interface{}) int64 {
	if a == nil || b == nil {
		var tmp interface{}
		if a == b {
			return 0
		} else if a == nil {
			tmp = b
		} else {
			tmp = a
		}
		//logger.Debug(tmp)
		switch tmp.(type) {
		case bool:
			return 1
		case int, int64:
			return xor(tmp, 0)
		case string:
			return xor(tmp, "")
		default:
			if !*fast {
				return xorGeneral(tmp, nil)
			}
			return 0
		}

		return 0
	}
	switch a.(type) {
	case bool:
		return xorBool(a.(bool), b.(bool))
	case int:
		return xorInt2(a.(int), b.(int))
	case int64:
		return xorInt64(a.(int64), b.(int64))
	case string:
		return xorString(a.(string), b.(string))
	default:
		if !*fast {
			return xorGeneral(a, b)
		}
		return 0
	}
	return 0
}

func equal(a, b interface{}) (diff int64) {
	defer func() {
		if r := recover(); r != nil {
			logger.Infof("Equality type check error caught return 0 diff %s\n", r)
		}
	}()
	diff = 0
	switch a.(type) {
	case map[string]interface{}:
		break
	default:
		if a == b {
			return diff + 1
		}
		break
	}
	return diff
}

func xorInt(a, b interface{}) int64 {
	var xorDiff int64
	switch a.(type) {
	case int:
		abig := big.NewInt(int64(a.(int)))
		bbig := big.NewInt(int64(b.(int)))
		z := big.NewInt(0)
		z.Xor(abig, bbig)
		for _, b := range z.Bytes() {
			xorDiff += int64(bitCounts[uint8(b)])
		}
		break
	default:
		break
	}
	return xorDiff
}

func runBadTests() {
	logger.Notice("running bad tests")
	t1 := xorInt2(0x00000000, 0x00000001)
	if t1 != 1 {
		logger.Errorf("error 0 xor 1 sould equal 1 not %d", t1)
	} else {
		logger.Debugf("0 xor 1 = %d", t1)
	}
	t1 = xorInt2(0x0000FF00, 0x0000F000)
	if t1 != 4 {
		logger.Errorf("error 0x0000FF00 xor 0x0000F000 sould equal 0x0000F000 not %x", t1)
	} else {
		logger.Debugf("0 xor 1 = %d", t1)
	}
	return
}

func xorInt2(a, b int) (xorDiff int64) {
	var x int
	var t uint8
	x = a ^ b
	for i := 0; i < 3; i++ {
		x, t = x>>8, uint8(x&0xff)
		xorDiff += int64(bitCounts[uint8(t)])
	}
	return
}

func xorInt64(a, b int64) (xorDiff int64) {
	var x int64
	var t uint8
	x = a ^ b
	for i := 0; i < 7; i++ {
		x, t = x>>8, uint8(x&0xff)
		xorDiff += int64(bitCounts[uint8(t)])
	}
	return
}

func xorString(a, b string) (xorDiff int64) {
	var i int
	for i = 0; i < len(a) && i < len(b); i++ {
		xorDiff += int64(bitCounts[a[i]^b[i]])
	}
	for ; i < len(a); i++ {
		xorDiff += 8
	}
	for ; i < len(b); i++ {
		xorDiff += 8
	}
	return

}

func xorBool(a, b bool) int64 {
	if a == b {
		return 0
	}
	return 1
}
func PrintStates(states []logmerger.State) {
	for _, state := range states {
		logger.Info(state.String())
	}
}

var bitCounts = []int8{
	// Generated by Java BitCount of all values from 0 to 255
	0, 1, 1, 2, 1, 2, 2, 3,
	1, 2, 2, 3, 2, 3, 3, 4,
	1, 2, 2, 3, 2, 3, 3, 4,
	2, 3, 3, 4, 3, 4, 4, 5,
	1, 2, 2, 3, 2, 3, 3, 4,
	2, 3, 3, 4, 3, 4, 4, 5,
	2, 3, 3, 4, 3, 4, 4, 5,
	3, 4, 4, 5, 4, 5, 5, 6,
	1, 2, 2, 3, 2, 3, 3, 4,
	2, 3, 3, 4, 3, 4, 4, 5,
	2, 3, 3, 4, 3, 4, 4, 5,
	3, 4, 4, 5, 4, 5, 5, 6,
	2, 3, 3, 4, 3, 4, 4, 5,
	3, 4, 4, 5, 4, 5, 5, 6,
	3, 4, 4, 5, 4, 5, 5, 6,
	4, 5, 5, 6, 5, 6, 6, 7,
	1, 2, 2, 3, 2, 3, 3, 4,
	2, 3, 3, 4, 3, 4, 4, 5,
	2, 3, 3, 4, 3, 4, 4, 5,
	3, 4, 4, 5, 4, 5, 5, 6,
	2, 3, 3, 4, 3, 4, 4, 5,
	3, 4, 4, 5, 4, 5, 5, 6,
	3, 4, 4, 5, 4, 5, 5, 6,
	4, 5, 5, 6, 5, 6, 6, 7,
	2, 3, 3, 4, 3, 4, 4, 5,
	3, 4, 4, 5, 4, 5, 5, 6,
	3, 4, 4, 5, 4, 5, 5, 6,
	4, 5, 5, 6, 5, 6, 6, 7,
	3, 4, 4, 5, 4, 5, 5, 6,
	4, 5, 5, 6, 5, 6, 6, 7,
	4, 5, 5, 6, 5, 6, 6, 7,
	5, 6, 6, 7, 6, 7, 7, 8,
}

func setupLogger() {
	// For demo purposes, create two backend for os.Stderr.
	backend := logging.NewLogBackend(os.Stderr, "", 0)

	// For messages written to backend2 we want to add some additional
	// information to the output, including the used log level and the name of
	// the function.
	backendFormatter := logging.NewBackendFormatter(backend, format)
	// Only errors and more severe messages should be sent to backend1
	backendlevel := logging.AddModuleLevel(backendFormatter)
	backendlevel.SetLevel(logging.Level(*loglevel), "")
	// Set the backends to be used.
	logging.SetBackend(backendlevel)
}

func setupProfiler() {
	//profiler setup
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			logger.Fatal("could not create CPU profile: ", err)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			logger.Fatal("could not start CPU profile: ", err)
		}
		defer pprof.StopCPUProfile()
	}
	if *memprofile != "" {
		f, err := os.Create(*memprofile)
		if err != nil {
			logger.Fatal("could not create memory profile: ", err)
		}
		runtime.GC() // get up-to-date statistics
		if err := pprof.WriteHeapProfile(f); err != nil {
			logger.Fatal("could not write memory profile: ", err)
		}
		f.Close()
	}
}
