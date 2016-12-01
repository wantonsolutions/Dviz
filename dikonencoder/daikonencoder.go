package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"strings"
)

const (
	INVSTART = 5
	INVEND   = 2
	regex    = `^(?P<LeftArgument>\S+) (?P<Operator>\S+) (?P<RightArgument>\S+)\n$`
)

type DaikonBinary struct {
	LeftArgument  string
	Operator      string
	RightArgument string
}

var logger *log.Logger

func main() {
	logger = log.New(os.Stdout, "[Dikon-To-Json] ", log.Lshortfile)
	if len(os.Args) != 2 {
		logger.Fatal("daikon encoder only takes a daikon invariant file as input")
	}
	fileBytes, err := ioutil.ReadFile(os.Args[1])
	if err != nil {
		logger.Fatal(err)
	}
	//convert the file into strings and split based on new lines
	fileString := string(fileBytes)
	split := strings.SplitAfter(fileString, "\n")
	trucatedFile := split[INVSTART : len(split)-INVEND]
	binaryInvariants := make([]DaikonBinary, 0)
	for _, inv := range trucatedFile {
		l, o, r, err := parseVariables(inv)
		//dont bother with inv's that dont meet the spec
		if err != nil {
			continue
		}
		binaryInvariants = append(binaryInvariants, DaikonBinary{LeftArgument: l, Operator: o, RightArgument: r})

	}
	output(binaryInvariants)

}

func parseVariables(invariant string) (string, string, string, error) {
	r := regexp.MustCompile(regex)
	res := r.FindStringSubmatch(invariant)
	//hardcoded for the
	//machine-package-filename-line-variable parsing
	logger.Printf("%#v\n", r.FindStringSubmatch(invariant))
	if len(res) != 4 {
		return "", "", "", fmt.Errorf("%s is not a binary invariant", invariant)
	}
	return res[1], res[2], res[3], nil
}

func output(bins []DaikonBinary) {
	outputJson, err := os.Create(os.Args[1] + ".json")
	if err != nil {
		logger.Fatal(err)
	}
	enc := json.NewEncoder(outputJson)
	enc.Encode(bins)
}
