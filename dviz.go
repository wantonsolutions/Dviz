package main

import (
	"os"
	"io"
	"log"
	"encoding/json"
	"encoding/gob"
	"regexp"
	"bytes"
    "math"
    "fmt"
    "os/exec"

	"bitbucket.org/bestchai/dinv/logmerger"
)

const (
	regex = `(?P<Host>[[:alpha:]]*)-(?P<Package>[[:alpha:]]*)_(?P<File>[[:alpha:]]*)_(?P<Line>[[:digit:]]*)_(?P<Name>[[:alnum:]]*)`
)

var (
	logger *log.Logger
	difference func (a , b interface{}) int64 
    output string
)

func main () {
	//read in states from a command line argument
	logger = log.New(os.Stdout,"Dviz:", log.Lshortfile)
	//set difference function
	difference = xor
    //set output filename
    output = "plot"
	if len(os.Args) != 2 {
		logger.Fatal("Supply a single state.json file as an argument")
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

	vectors := stateVectors(states)
	velocity := diff(vectors)
	mag := magnitude(velocity)
    dat(mag)
    gnuplot(mag)
    render()
}


func stateVectors(states []logmerger.State) map[string]map[string][]interface{} {
	r := regexp.MustCompile(regex)
	hostVectors := make(map[string]map[string][]interface{},0)
	for _, state := range states {
		for _, point := range state.Points {
			for _, variable := range point.Dump {
				//logger.Printf("%#v\n", r.FindStringSubmatch(variable.VarName))
				res := r.FindStringSubmatch(variable.VarName)
				//hardcoded for the
				//machine-package-filename-line-variable parsing
				if len(res) != 6 {
					logger.Fatalf("regex unable to parse variable %s\n",variable.VarName)
				}
				_, ok := hostVectors[res[1]]
				if !ok {
					hostVectors[res[1]] = make(map[string][]interface{},0)
				}
				_, ok = hostVectors[res[1]][res[2]+res[3]+res[4]+res[5]]
				if !ok {
					hostVectors[res[1]][res[2]+res[3]+res[4]+res[5]] = make([]interface{},0)
				}
				hostVectors[res[1]][res[2]+res[3]+res[4]+res[5]] = append(hostVectors[res[1]][res[2]+res[3]+res[4]+res[5]],variable.Value)

				
			}
		}
	}
	return hostVectors
}

func diff(vectors map[string]map[string][]interface{} ) map[string]map[string][]int64 {
	diff := make(map[string]map[string][]int64,0)
	for host := range vectors {
		diff[host] = make(map[string][]int64,0)
		for variable := range vectors[host] {
			diff[host][variable] = make([]int64,len(vectors[host][variable])-1)
			//comparing two values means that we stop one element
			//short
			for i := 0; i <len(vectors[host][variable])-1; i++ {
				diff[host][variable][i] = difference(vectors[host][variable][i],vectors[host][variable][i+1])
			}
		}
	}
	return diff
}

func magnitude(velocity map[string]map[string][]int64) map[string][]float64 {
	mag := make(map[string][]float64,0)
	for host := range velocity {
		mag[host] = make([]float64,0)
		for stubVar := range velocity[host] {
			length := len(velocity[host][stubVar])
			var ithMag float64
			for i:= 0; i<length; i++ {
				for variable := range velocity[host] {
					ithMag += float64(velocity[host][variable][i]) * float64(velocity[host][variable][i])
				}
                mag[host] = append(mag[host],math.Sqrt(ithMag))
			}
            
        }
    }
    return mag
}
//set term png
//set output "plot.png"

//plot "output.dat" using 2 title 'apple' with lines, \
//     "output.dat" using 3 title 'apricot' with lines

func gnuplot(magnitude map[string][]float64) {
    f, err := os.Create(output+ ".plot")
    if err != nil {
        logger.Fatal(err)
    }
    f.WriteString("set term png\n")
    f.WriteString("set output \""+output+".png\"\n")
    f.WriteString("set xlabel \"Vector Time\"\n")
    f.WriteString("set ylabel \"State Velocity\"\n")
    f.WriteString("plot ")
    i := 0
    for host := range magnitude {
        f.WriteString(fmt.Sprintf("\"%s.dat\" using %d title '%s' with lines",output,i+2,host))
        if i+1 != len(magnitude) {
            f.WriteString(", \\\n\t")
        }
        i++
    }
}

func dat(magnitude map[string][]float64) {
    f, err := os.Create(output+ ".dat")
    if err != nil {
        logger.Fatal(err)
    }
    var length int
    for host := range magnitude {
        length = len(magnitude[host])
        break
    }
    for i := 0; i < length; i++ {
        f.WriteString(fmt.Sprintf("%d\t",i))
        for host := range magnitude {
            f.WriteString(fmt.Sprintf("%f\t",magnitude[host][i]))
        }
        f.WriteString("\n")
    }
}
        


func render() {
    cmd := exec.Command("gnuplot", output+".plot")
    if err := cmd.Run(); err != nil {
        logger.Fatal(err)
    }
    cmd = exec.Command("display", output+".png")
    if err := cmd.Run(); err != nil {
        logger.Fatal(err)
    }
}



func xor (a , b interface{}) int64 {

	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
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

	var xorDiff int64
	var i int
	for i =0; i< len(abytes) && i < len(bbytes); i++ {
		if abytes[i] != bbytes[i] {
			xorDiff++
		}
	}
	for ;i < len(abytes); i++ {
		xorDiff++
	}
	for ;i < len(bbytes); i++ {
		xorDiff++
	}
	return xorDiff
}





func PrintStates( states []logmerger.State) {
	for _, state := range states {
		logger.Println(state.String())
	}
}

