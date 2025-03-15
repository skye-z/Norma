package main

import (
	"flag"
	"fmt"
	"os"

	"norma/internal/live2d"
)

func main() {
	dir := flag.String("dir", "resource/model", "live2d model dir")
	flag.Parse()

	res := live2d.ValidateModelDir(*dir)
	fmt.Printf("ModelDir: %s\n", res.ModelDir)
	fmt.Printf("HasCan3: %v\n", res.HasCan3)
	fmt.Printf("HasModel3JSON: %v\n", res.HasModel3JSON)
	fmt.Printf("HasCmo3: %v\n", res.HasCmo3)

	if res.OK() {
		fmt.Println("Status: OK")
		return
	}

	fmt.Println("Status: FAILED")
	for i, issue := range res.Issues {
		fmt.Printf("%d. %s\n", i+1, issue)
	}
	os.Exit(1)
}
