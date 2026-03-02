package controllergen

import (
	"fmt"
	"go/ast"
	"go/types"
	"strings"
	"unicode"

	"sigs.k8s.io/controller-tools/pkg/genall"
	"sigs.k8s.io/controller-tools/pkg/loader"
	"sigs.k8s.io/controller-tools/pkg/markers"
)

// WranglerGenerator is a genall.Generator that produces Wrangler controllers.
type WranglerGenerator struct {
	// OutputPackage is the base Go package for the generated code.
	OutputPackage string `marker:"outputPackage,optional"`
	// Boilerplate is the path to a file containing the Go header boilerplate.
	Boilerplate string `marker:"boilerplate,optional"`
}

func (g *WranglerGenerator) RegisterMarkers(into *markers.Registry) error {
	if err := into.Register(GenerateMarker); err != nil {
		return err
	}
	if err := into.Register(GroupMarker); err != nil {
		return err
	}
	return nil
}

func (g *WranglerGenerator) CheckFilter() loader.NodeFilter {
	return func(node ast.Node) bool {
		return true
	}
}

func (g *WranglerGenerator) Generate(ctx *genall.GenerationContext) error {
	for _, root := range ctx.Roots {
		pkgMarkers, err := markers.PackageMarkers(ctx.Collector, root)
		if err != nil {
			return err
		}
		pkgMarker := pkgMarkers.Get(GroupMarker.Name)
		if pkgMarker == nil {
			continue
		}

		groupName := pkgMarker.(Group).Name
		fmt.Printf("Generating Wrangler controllers for group: %s (in %s)\n", groupName, root.PkgPath)

		err = markers.EachType(ctx.Collector, root, func(info *markers.TypeInfo) {
			if marker := info.Markers.Get(GenerateMarker.Name); marker != nil {
				fmt.Printf("  Found type: %s\n", info.Name)
				// Verification: Check if type info is available
				if root.TypesInfo != nil {
					if obj := root.TypesInfo.Defs[info.RawSpec.Name]; obj != nil {
						if _, ok := obj.Type().Underlying().(*types.Struct); ok {
							// Successfully parsed struct info
						}
					}
				}
			}
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (g *WranglerGenerator) Help() *markers.Definition {
	return markers.Must(markers.MakeDefinition("wrangler", markers.DescribesPackage, WranglerGenerator{}))
}

// Utility functions for string manipulation
func upperFirst(s string) string {
	if s == "" {
		return ""
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

func lowerFirst(s string) string {
	if s == "" {
		return ""
	}
	r := []rune(s)
	r[0] = unicode.ToLower(r[0])
	return string(r)
}

func pluralize(name string) string {
	// Basic pluralization for now, will refine in later phases
	if strings.HasSuffix(name, "s") {
		return name
	}
	if strings.HasSuffix(name, "y") {
		return name[:len(name)-1] + "ies"
	}
	return name + "s"
}
