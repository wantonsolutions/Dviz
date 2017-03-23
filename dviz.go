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
	"github.com/sacado/tsne4go"
	//"encoding/gob"
	"bytes"
	"fmt"
	"math"
	"math/big"
	"os/exec"
	"regexp"
	"strings"

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
	tsneItt    = flag.Int("itt", 400, "tsne itterations, more increase runitme")
	draw       = flag.Bool("d", false, "draw clustered scatterplot")

	difference   func(a, b interface{}) int64
	render       string
	total        int
	targetMisses int
	badTesting   = false
)

type Query struct {
}

type StatePlane struct {
	States      []State
	Plane       [][]float64
	Points      []tsne4go.Point
	NumClusters int
}

//State of a distributed program at a moment corresponding to a cut
type State struct {
	//Dinv Data
	Cut           logmerger.Cut
	Points        []logmerger.Point
	TotalOrdering [][]int
	//Viz Data
	ClusterId int
}

func (s State) String() string {
	return fmt.Sprintf("%s,%s,%s,%s", s.Cut.String(), s.Points, s.TotalOrdering, s.ClusterId)
}

func (sp StatePlane) Len() int { return len(sp.Plane) }

func (sp StatePlane) Distance(i, j int) float64 { return sp.Plane[i][j] }

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
	plane := dviz(states)
	//TODO come up with a proper naming scheme
	output(plane, *outputfile)
	logger.Debugf("target misses %d\n", targetMisses)
	logger.Debugf("target ration %f\n", float32(targetMisses)/float32(total))

	//plot the single dimension version

	/*
		if *draw {
			render = "default"
			dat(plane)
			gnuplotPlane()
			renderImage()
		}
	*/

}

func decodeAndCorrect(jsonFile io.ReadCloser) []State {
	dec := json.NewDecoder(jsonFile)
	states := make([]State, 0)
	//states2 := make([]State, 0)
	var err error
	err = nil
	for err == nil {
		var decodedState State
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

func dviz(states []State) *StatePlane {
	dplane := dvizMaster2(&states)
	sp := StatePlane{States: states, Plane: dplane, Points: make([]tsne4go.Point, 0)}
	/*
		tsne := tsne4go.New(sp, nil)
		for i := 0; i < *tsneItt; i++ {
			tsne.Step()
			//logger.Debugf("cost %d", cost)
		}
		tsne.NormalizeSolution()
		s := tsne.Solution
		//logger.Debugf("%d", s[0][0])
		for i := 0; i < len(s); i++ {
			logger.Debugf("point: %g %g", s[i][0], s[i][1])
		}
		sp.Points = s

		//Entry point for goXmeans
		f, err := os.Create("goxmeans.input")
		if err != nil {
			logger.Fatal(err)
		}
		for i := range sp.Points {
			//fmt.Printf("to write %g",sp.Points[i][0])
			f.WriteString(fmt.Sprintf("%g,%g\n", sp.Points[i][0], sp.Points[i][1]))
		}
		//copy paste goXmeans
		data, err := goxmeans.Load("goxmeans.input", ",")
		if err != nil {
			logger.Fatalf("Load: ", err)
		}
		fmt.Println("Load complete")

		var k = 2
		var kmax = 6
		// Type of data measurement between points.
		var measurer goxmeans.EuclidDist
		// How to select your initial centroids.
		var cc goxmeans.DataCentroids
		// How to select centroids for bisection.
		bisectcc := goxmeans.EllipseCentroids{1.0}
		// Initial matrix of centroids to use
		centroids := cc.ChooseCentroids(data, k)
		models, errs := goxmeans.Xmeans(data, centroids, k, kmax, cc, bisectcc, measurer)
		if len(errs) > 0 {
			for k, v := range errs {
				fmt.Printf("%s: %v\n", k, v)
			}
		}
		// Find and display the best model
		bestbic := math.Inf(-1)
		bestidx := 0
		for i, m := range models {
			if m.Bic > bestbic {
				bestbic = m.Bic
				bestidx = i
			}
			logger.Debugf("%d: #centroids=%d BIC=%f\n", i, m.Numcentroids(), m.Bic)
		}
		logger.Debugf("\nBest fit:[ %d: #centroids=%d BIC=%f]\n", bestidx, models[bestidx].Numcentroids(), models[bestidx].Bic)
		bestm := models[bestidx]
		sp.NumClusters = len(bestm.Clusters)
		for i, c := range bestm.Clusters {
			logger.Debugf("cluster-%d: numpoints=%d variance=%f\n", i, c.Numpoints(), c.Variance)
			//logger.Debugf("%s",c.Points.String())
		}

		clusterMap := make(map[tsne4go.Point]int, 0)
		for i, c := range bestm.Clusters {
			for j := 0; j < c.Points.Rows(); j++ {
				x, y := c.Points.Get(j, 0), c.Points.Get(j, 1)
				clusterMap[tsne4go.Point{x, y}] = i
			}
		}

		for i, point := range sp.Points {
			sp.States[i].ClusterId = clusterMap[point]
		}

		ClusterInvariants(&sp)
	*/
	return &sp
}

func trim64point(p64 tsne4go.Point) tsne4go.Point {
	p64[0], p64[1] = float64(float32(p64[0])), float64(float32(p64[1]))
	return p64
}

func output(sp *StatePlane, outputfile string) {
	outputJson, err := os.Create(outputfile)
	if err != nil {
		logger.Fatal(err)
	}
	enc := json.NewEncoder(outputJson)
	enc.Encode(*sp)
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
func TypeCorrectJson(states *[]State) {
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

type Index2 struct {
	X    int
	Y    int
	Diff float64
}

func dvizMaster2(states *[]State) [][]float64 {
	//get state array
	var length = len(*states) - 1
	//real algorithm starts here
	plane := make([][]float64, length)
	for i := 0; i < length; i++ {
		plane[i] = make([]float64, length)
	}
	//launch threads
	input := make(chan Index2, 1000)
	output := make(chan Index2, 1000)
	for i := 0; i < runtime.NumCPU(); i++ {
		go distanceWorker2(states, input, output)
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

	return nil
}

func distanceWorker2(states *[]State, input chan Index2, output chan Index2) {
	for true {
		index := <-input
		var runningDistance int64
		var dVar int64
		for i := range (*states)[index.X].Points {
			for j := range (*states)[index.X].Points[i].Dump {
				if len((*states)[index.Y].Points) != len((*states)[index.X].Points) {
					//TODO see if this is systematic if so do len < 1 and dont bother checking
					continue
				}
				dVar = difference((*states)[index.X].Points[i].Dump[j].Value, (*states)[index.Y].Points[i].Dump[j].Value)
				runningDistance += dVar * dVar
				//total++
			}
		}
		index.Diff = math.Sqrt(float64(runningDistance))
		output <- index
	}
}

func gnuplotPlane() {
	f, err := os.Create(render + ".plot")
	if err != nil {
		logger.Fatal(err)
	}
	f.WriteString("set term pdf\n")
	f.WriteString("set output \"" + render + ".pdf\"\n")
	f.WriteString("set title \"DviZ\"\n")
	f.WriteString(fmt.Sprintf("plot \"%s.dat\" with points palette, \\\n", render))
	//f.WriteString(fmt.Sprintf("\t '' using 1:2 w linespoints\n"))
	//plot "default.dat" with points palette, \
	//    '' using 1:2 w linespoints

}

func dat(sp *StatePlane) {
	f, err := os.Create(render + ".dat")
	if err != nil {
		logger.Fatal(err)
	}
	for i := range (*sp).Points {
		f.WriteString(fmt.Sprintf("%f %f %d\n", sp.Points[i][0], sp.Points[i][1], sp.States[i].ClusterId))
	}
}

func renderImage() {
	cmd := exec.Command("gnuplot", render+".plot")
	if err := cmd.Run(); err != nil {
		logger.Fatal(err)
	}
	/*
		cmd = exec.Command("display", render+".png")
		if err := cmd.Run(); err != nil {
			logger.Fatal(err)
		}
	*/
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
func PrintStates(states []State) {
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

func ClusterInvariants(sp *StatePlane) {
	//build daikon files
	clusterLogs := make([][]logmerger.Point, sp.NumClusters)
	for i := range sp.States {
		//bucket clusters of points into seperate files. Merge the points of individual states together.
		clusterLogs[sp.States[i].ClusterId] = append(clusterLogs[sp.States[i].ClusterId], logmerger.MergePoints(sp.States[i].Points))
	}
	//TODO filter based on hosts and variables
	for i := range clusterLogs {
		logger.Debug("writing to daikon log")
		logmerger.WriteToDaikon(clusterLogs[i], fmt.Sprintf("c%d", i))
	}
	//execute daikon collect Invariants

	clusterinvs := make([]map[string]bool, sp.NumClusters)
	for i := range clusterLogs {
		cmd := exec.Command("java", "daikon.Daikon", fmt.Sprintf("c%d.dtrace", i))
		stdoutStderr, err := cmd.CombinedOutput()
		if err != nil {
			logger.Fatal(err)
		}
		//logger.Debug("%s\n",string(stdoutStderr))
		clusterinvs[i] = make(map[string]bool)
		invs := strings.SplitAfter(string(stdoutStderr), "\n")
		//Avoid the first and last line while building maps, they contain text which is not invaraints
		for j := 3; j < len(invs)-1; j++ {
			clusterinvs[i][invs[j]] = true
		}
		logger.Debug(clusterinvs)
		//split up invariants into individual lines and store them
	}

}
