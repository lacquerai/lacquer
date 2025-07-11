package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"time"
)

type Input struct {
	Inputs Inputs `json:"inputs"`
}

type Inputs struct {
	Name string `json:"name"`
}

func main() {
	stdin, err := io.ReadAll(os.Stdin)
	if err != nil {
		log.Fatal(err)
	}

	var input Input
	err = json.Unmarshal(stdin, &input)
	if err != nil {
		log.Fatal(err)
	}

	time.Sleep(time.Second * 3)

	json.NewEncoder(os.Stdout).Encode(map[string]interface{}{
		"greeting": fmt.Sprintf("Hello, %s!", input.Inputs.Name),
	})
}
