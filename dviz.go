package main

import (
	"os"
	"io"
	"log"
	"encoding/json"

	"bitbucket.org/bestchai/dinv/logmerger"
)

const (
	regex = ""
)

var (
	logger *log.Logger
)

func main () {
	//read in states from a command line argument
	logger = log.New(os.Stdout,"Dviz:", log.Lshortfile)
	logger.Printf("#Command Line Arguments %d",len(os.Args))
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
	PrintStates(states)


}

func PrintStates( states []logmerger.State) {
	for _, state := range states {
		logger.Println(state.String())
	}
}
