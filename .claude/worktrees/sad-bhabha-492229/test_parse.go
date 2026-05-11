package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type popeyeJSON struct {
	Popeye struct {
		Score    int    `json:"score"`
		Grade    string `json:"grade"`
		Sections []struct {
			Linter string `json:"linter"`
			Tally  struct {
				OK   int `json:"ok"`
				Info int `json:"info"`
				Warn int `json:"warning"`
				Err  int `json:"error"`
			} `json:"tally"`
			Issues map[string][]struct {
				Group   string `json:"group"`
				Level   int    `json:"level"`
				Message string `json:"message"`
			} `json:"issues"`
		} `json:"sections"`
	} `json:"popeye"`
}

func main() {
	b, err := os.ReadFile("popeye_out.json")
	if err != nil {
		panic(err)
	}
	var raw popeyeJSON
	if err := json.Unmarshal(b, &raw); err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println("Success! Score:", raw.Popeye.Score)
}
