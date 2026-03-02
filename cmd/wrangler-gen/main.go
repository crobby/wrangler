package main

import (
	"fmt"
	"os"

	"github.com/rancher/wrangler/v3/pkg/controller-gen"
	"sigs.k8s.io/controller-tools/pkg/genall"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: wrangler-gen <package-path>")
		os.Exit(1)
	}
	roots := os.Args[1:]

	var g genall.Generator = &controllergen.WranglerGenerator{}
	allGenerators := genall.Generators{&g}

	// In a full implementation, we'd use genall.FromOptions for more complex
	// flags, but for now we manually set up the runtime with our roots.
	runtime, err := allGenerators.ForRoots(roots...)
	if err != nil {
		fmt.Printf("Error setting up runtime: %v\n", err)
		os.Exit(1)
	}

	if !runtime.Run() {
		fmt.Println("Generation failed")
		os.Exit(1)
	}

	fmt.Println("Generation complete")
}
