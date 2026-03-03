package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/rancher/wrangler/v3/pkg/controller-gen"
	"sigs.k8s.io/controller-tools/pkg/crd"
	"sigs.k8s.io/controller-tools/pkg/deepcopy"
	"sigs.k8s.io/controller-tools/pkg/genall"
)

func main() {
	var (
		generateDeepcopy bool
		generateCRD      bool
		outputPackage    string
		boilerplate      string
	)

	flag.BoolVar(&generateDeepcopy, "deepcopy", false, "Generate deepcopy methods")
	flag.BoolVar(&generateCRD, "crd", false, "Generate CRDs")
	flag.StringVar(&outputPackage, "output-package", "github.com/rancher/wrangler/v3/pkg/generated", "Output package")
	flag.StringVar(&boilerplate, "boilerplate", "scripts/boilerplate.go.txt", "Boilerplate file")
	flag.Parse()

	roots := flag.Args()
	if len(roots) == 0 {
		fmt.Println("Usage: wrangler-gen [flags] <package-path>...")
		flag.PrintDefaults()
		os.Exit(1)
	}

	var (
		wranglerGen genall.Generator = &controllergen.WranglerGenerator{
			OutputPackage: outputPackage,
			Boilerplate:   boilerplate,
		}
		deepcopyGen genall.Generator = &deepcopy.Generator{}
		crdGen      genall.Generator = &crd.Generator{}
	)
	
	gens := genall.Generators{&wranglerGen}
	if generateDeepcopy {
		gens = append(gens, &deepcopyGen)
	}
	if generateCRD {
		gens = append(gens, &crdGen)
	}

	runtime, err := gens.ForRoots(roots...)
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
