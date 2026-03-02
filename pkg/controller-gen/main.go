package controllergen

import (
	"fmt"
	"os"

	"github.com/rancher/wrangler/v3/pkg/controller-gen/args"
	"sigs.k8s.io/controller-tools/pkg/crd"
	"sigs.k8s.io/controller-tools/pkg/deepcopy"
	"sigs.k8s.io/controller-tools/pkg/genall"
)

func Run(opts args.Options) {
	var (
		wranglerGen genall.Generator = &WranglerGenerator{
			OutputPackage: opts.OutputPackage,
			Boilerplate:   opts.Boilerplate,
			ManualGroups:  opts.Groups,
		}
		deepcopyGen genall.Generator = &deepcopy.Generator{}
		crdGen      genall.Generator = &crd.Generator{}
	)

	var (
		gens            genall.Generators
		generateTypes   bool
		generateOpenAPI bool
	)

	gens = append(gens, &wranglerGen)

	for _, group := range opts.Groups {
		if group.GenerateTypes {
			generateTypes = true
		}
		if group.GenerateOpenAPI {
			generateOpenAPI = true
		}
	}

	if generateTypes {
		gens = append(gens, &deepcopyGen)
	}
	if generateOpenAPI {
		gens = append(gens, &crdGen)
	}

	runtime, err := gens.ForRoots()
	if err != nil {
		fmt.Printf("Error setting up runtime: %v\n", err)
		os.Exit(1)
	}
	runtime.ErrorWriter = os.Stderr
	
	// Only set OutputRules if we actually have something to output to Config
	if generateOpenAPI {
		runtime.OutputRules = genall.OutputRules{
			Default: genall.OutputArtifacts{
				Config: genall.OutputToDirectory("pkg/crds"),
			},
		}
	}

	if runtime.Run() {
		fmt.Println("Generation failed")
		os.Exit(1)
	}

	fmt.Println("Generation complete")
}
