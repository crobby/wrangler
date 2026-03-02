package main

import (
	"fmt"
	"os"

	"github.com/rancher/wrangler/v3/pkg/controller-gen"
	"sigs.k8s.io/controller-tools/pkg/crd"
	"sigs.k8s.io/controller-tools/pkg/deepcopy"
	"sigs.k8s.io/controller-tools/pkg/genall"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: wrangler-gen <package-path>")
		os.Exit(1)
	}
	roots := os.Args[1:]

	var (
		wranglerGen genall.Generator = &controllergen.WranglerGenerator{
			OutputPackage: "github.com/rancher/wrangler/v3/pkg/generated",
			Boilerplate:   "scripts/boilerplate.go.txt",
		}
		deepcopyGen genall.Generator = &deepcopy.Generator{}
		crdGen      genall.Generator = &crd.Generator{}
	)
	
	allGenerators := genall.Generators{&wranglerGen, &deepcopyGen, &crdGen}

	// In a full implementation, we'd use genall.FromOptions for more complex
	// flags, but for now we manually set up the runtime with our roots.
	runtime, err := allGenerators.ForRoots(roots...)
	if err != nil {
		fmt.Printf("Error setting up runtime: %v\n", err)
		os.Exit(1)
	}
	runtime.ErrorWriter = os.Stderr
	runtime.OutputRules = genall.OutputRules{
		Default: genall.OutputArtifacts{
			Config: genall.OutputToDirectory("pkg/crds"),
		},
	}

	if runtime.Run() {
		fmt.Println("Generation failed")
		for _, root := range runtime.Roots {
			if len(root.Errors) > 0 {
				for _, err := range root.Errors {
					fmt.Printf("Error in package %s: %v\n", root.PkgPath, err)
				}
			}
		}
		os.Exit(1)
	}

	fmt.Println("Generation complete")
}
