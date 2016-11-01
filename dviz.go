package main

import (
	"os"
	"io"
	"log"
	"encoding/json"
	//"encoding/gob"
	"regexp"
	"bytes"
    "math"
    "math/big"
    "fmt"
    "os/exec"

	"bitbucket.org/bestchai/dinv/logmerger"
)

const (
	regex = `(?P<Host>[[:alpha:]]*)-(?P<Package>[[:alpha:]]*)_(?P<File>[[:alpha:]]*)_(?P<Line>[[:digit:]]*)_(?P<Name>[[:alnum:]]*)`
	regex2 = `(?P<Host>[[:alnum:]]*)-(?P<Name>.*)`
)

var (
	logger *log.Logger
	difference func (a , b interface{}) int64 
	draw = false
	render string
)

type StatePlane struct {
	States []logmerger.State
	Plane [][]float64
}

func main () {
	//read in states from a command line argument
	logger = log.New(os.Stdout,"Dviz:", log.Lshortfile)
	//set difference function
	difference = xorInt
	if len(os.Args) != 3 {
		logger.Fatal("Supply an input.json and output.json")
	}
	filename := os.Args[1]
	jsonFile, err := os.Open(filename)
	if err != nil {
		logger.Fatal(err)
	}
	dec := json.NewDecoder(jsonFile)
	states := make([]logmerger.State,0)
	err = nil
	for err == nil {
		var decodedState logmerger.State
		err = dec.Decode(&decodedState)
		if err != nil && err != io.EOF {
			logger.Fatal(err)
		}
		states = append(states,decodedState)
	}
    if len(states) <= 2 {
        logger.Fatal("error: not enough states to produce a plot\n")
    }


	TypeCorrectJson(&states)
	vectors := stateVectors(states)
	plane := diff(vectors)
	dplane := mag(plane)
	output(StatePlane{States: states, Plane: dplane},os.Args[2])

	//plot the single dimension version
	if draw {
		render = "default"
		dat(dplane)
		gnuplotPlane()
		renderImage()
	}
}

func output(stateplane StatePlane, outputfile string){
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
		logger.Fatalf("regex unable to parse variable %s\n",name)
	}
	return res[1], res[2]+res[3]+res[4]+res[5]
}

func parseVariables2(name string) (string, string) {
	r := regexp.MustCompile(regex2)
	res := r.FindStringSubmatch(name)
	//hardcoded for the
	//machine-package-filename-line-variable parsing
	//logger.Printf("%#v\n", r.FindStringSubmatch(name))
	if len(res) != 3 {
		logger.Fatalf("regex unable to parse variable %s\n",name)
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

func stateVectors(states []logmerger.State) map[string]map[string][]interface{} {
	hostVectors := make(map[string]map[string][]interface{},0)
	for _, state := range states {
		for _, point := range state.Points {
			for _, variable := range point.Dump {
				host, name := parseVariables2(variable.VarName)
				_, ok := hostVectors[host]
				if !ok {
					hostVectors[host] = make(map[string][]interface{},0)
				}
				_, ok = hostVectors[host][name]
				if !ok {
					hostVectors[host][name] = make([]interface{},0)
				}
				hostVectors[host][name] = append(hostVectors[host][name],variable.Value)

				
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
	plane := make([][][]int64,length)
	for i:= 0; i<length;i++ {
		plane[i] = make([][]int64,length)
		for j:=0; j<length;j++ {
			fmt.Printf("\rCalculating Diff %3.0f%%",100*(float32(i+1)/float32(len(plane))))
			plane[i][j] = make([]int64,0)
			for host := range vectors {
				for variable := range vectors[host] {
					plane[i][j] = append(plane[i][j],difference(vectors[host][variable][i],vectors[host][variable][j]))
				}
			}
		}
	}
	fmt.Println()
	return plane

}

func mag(plane [][][]int64) [][]float64 {
	dplane := make([][]float64,len(plane))
	for i := range plane {
		dplane[i] = make([]float64,len(plane))
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


func gnuplotPlane(){
    f, err := os.Create(render+ ".plot")
    if err != nil {
        logger.Fatal(err)
    }
    f.WriteString("set term png\n")
    f.WriteString("set output \""+render+".png\"\n")
	f.WriteString(fmt.Sprintf("plot \"%s.dat\" matrix with image\n",render))

}

func dat(dplane [][]float64){
    f, err := os.Create(render+ ".dat")
    if err != nil {
        logger.Fatal(err)
    }
	for i:= range dplane {
		//f.WriteString(fmt.Sprintf("%d\t",i))
		for j:= range dplane[i]{
			f.WriteString(fmt.Sprintf("%f\t",dplane[i][j]))
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



func xor (a , b interface{}) int64 {
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
	for i =0; i< len(abytes) && i < len(bbytes); i++ {
		abig := big.NewInt(int64(abytes[i]))
		bbig := big.NewInt(int64(bbytes[i]))
		z := big.NewInt(0)
		z.Xor(abig,bbig)
		for _, bits := range z.Bits() {
        	xorDiff += int64(bitCounts[bits])
    	}
	}
	for ;i < len(abytes); i++ {
		xorDiff+=8
	}
	for ;i < len(bbytes); i++ {
		xorDiff+=8
	}
	return xorDiff
}

func xorInt (a , b interface{}) int64 {
	var xorDiff int64
	switch a.(type) {
	case int:
		abig := big.NewInt(int64(a.(int)))
		bbig := big.NewInt(int64(b.(int)))
		z := big.NewInt(0)
		z.Xor(abig,bbig)
		for _, b := range z.Bytes() {
			xorDiff += int64(bitCounts[uint8(b)])
		}
		break
	default:
		break
	}
	return xorDiff
}


func PrintStates( states []logmerger.State) {
	for _, state := range states {
		logger.Println(state.String())
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
