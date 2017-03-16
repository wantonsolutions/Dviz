package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
)

func main() {
	filename := os.Args[1]
	jsonFile, _ := os.Open(filename)
	resp, err := http.Post("http://13.64.149.118:3119/", "application/json", jsonFile)
	out, err := os.Create("output.json")
	r, err := ioutil.ReadAll(resp.Body)
	out.Write(r)
	fmt.Printf("CLIENT: %s\n %s", resp.Body, err)
}
