package controllergen

import (
	"fmt"
	"os"

	"github.com/rancher/wrangler/v3/pkg/controller-gen/args"
	"sigs.k8s.io/controller-tools/pkg/genall"
)

func Run(opts args.Options) {
	g := &WranglerGenerator{
		OutputPackage: opts.OutputPackage,
		Boilerplate:   opts.Boilerplate,
		ManualGroups:  opts.Groups,
	}

	var gen genall.Generator = g
	allGenerators := genall.Generators{&gen}

	// We don't have roots in the classic Run(opts), so we just run against
	// the manual groups.
	runtime, err := allGenerators.ForRoots()
	if err != nil {
		fmt.Printf("Error setting up runtime: %v\n", err)
		os.Exit(1)
	}
	runtime.ErrorWriter = os.Stderr

	if runtime.Run() {
		fmt.Println("Generation failed")
		os.Exit(1)
	}

	fmt.Println("Generation complete")
}
