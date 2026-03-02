package controllergen

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/types"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/tools/imports"
	"sigs.k8s.io/controller-tools/pkg/genall"
	"sigs.k8s.io/controller-tools/pkg/loader"
	"sigs.k8s.io/controller-tools/pkg/markers"

	"github.com/rancher/wrangler/v3/pkg/controller-gen/args"
)

// WranglerGenerator is a genall.Generator that produces Wrangler controllers.
type WranglerGenerator struct {
	// OutputPackage is the base Go package for the generated code.
	OutputPackage string `marker:"outputPackage,optional"`
	// Boilerplate is the path to a file containing the Go header boilerplate.
	Boilerplate string `marker:"boilerplate,optional"`
	// ManualGroups allows specifying groups and types manually instead of via markers.
	ManualGroups map[string]args.Group
}

func (g *WranglerGenerator) RegisterMarkers(into *markers.Registry) error {
	if err := into.Register(GenerateMarker); err != nil {
		return err
	}
	if err := into.Register(GroupMarker); err != nil {
		return err
	}
	if err := into.Register(ClientsetMarker); err != nil {
		return err
	}
	if err := into.Register(ListersMarker); err != nil {
		return err
	}
	if err := into.Register(InformersMarker); err != nil {
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
	var boilerplate string
	if g.Boilerplate != "" {
		content, err := os.ReadFile(g.Boilerplate)
		if err != nil {
			return err
		}
		boilerplate = string(content)
	}

	metadata := &GenerationMetadata{}
	groups := make(map[string]*GroupMetadata)

	if len(g.ManualGroups) > 0 {
		if err := g.collectManualMetadata(ctx, groups, metadata); err != nil {
			return err
		}
	}

	for _, root := range ctx.Roots {
		if ctx.Checker != nil {
			ctx.Checker.Check(root)
		} else {
			root.NeedTypesInfo()
		}
		pkgMarkers, err := markers.PackageMarkers(ctx.Collector, root)
		if err != nil {
			return err
		}
		pkgMarker := pkgMarkers.Get(GroupMarker.Name)
		if pkgMarker == nil {
			continue
		}

		groupName := pkgMarker.(Group).Name
		customPkg := pkgMarker.(Group).PackageName
		metadata.GenerateClientset = metadata.GenerateClientset || pkgMarkers.Get(ClientsetMarker.Name) != nil
		metadata.GenerateListers = metadata.GenerateListers || pkgMarkers.Get(ListersMarker.Name) != nil
		metadata.GenerateInformers = metadata.GenerateInformers || pkgMarkers.Get(InformersMarker.Name) != nil

		group, ok := groups[groupName]
		if !ok {
			pkgName := customPkg
			if pkgName == "" {
				pkgName = strings.ReplaceAll(groupName, "-", "")
				if pkgName == "" {
					pkgName = "core"
				}
			}
			group = &GroupMetadata{
				Name:              groupName,
				UpperName:         upperFirst(strings.ReplaceAll(pkgName, ".", "")),
				PackageName:       pkgName,
				CustomPackageName: customPkg,
			}
			groups[groupName] = group
		}

		versionName := root.Name
		version := VersionMetadata{
			Version:      versionName,
			VersionUpper: upperFirst(versionName),
			TypesPkg:     root.PkgPath,
		}

		err = markers.EachType(ctx.Collector, root, func(info *markers.TypeInfo) {
			if marker := info.Markers.Get(GenerateMarker.Name); marker != nil {
				typeMetadata := TypeMetadata{
					Name:        info.Name,
					LowerName:   lowerFirst(info.Name),
					Plural:      pluralize(info.Name),
					PluralLower: strings.ToLower(pluralize(info.Name)),
					Group:       groupName,
					Version:     versionName,
				}

				// Check for namespacing markers
				typeMetadata.Namespaced = true
				for _, markerValues := range info.Markers {
					for _, val := range markerValues {
						if s, ok := val.(string); ok {
							if strings.Contains(s, "nonNamespaced") || strings.Contains(s, "scope=Cluster") {
								typeMetadata.Namespaced = false
							}
						}
					}
				}

				// Check for Status field
				if root.TypesInfo != nil {
					if obj := root.TypesInfo.Defs[info.RawSpec.Name]; obj != nil {
						if structType, ok := obj.Type().Underlying().(*types.Struct); ok {
							for i := 0; i < structType.NumFields(); i++ {
								field := structType.Field(i)
								if field.Name() == "Status" {
									typeMetadata.HasStatus = true
									typeMetadata.StatusType = field.Type().String()
									if strings.Contains(typeMetadata.StatusType, ".") {
										parts := strings.Split(typeMetadata.StatusType, ".")
										typeMetadata.StatusType = parts[len(parts)-1]
									}
									break
								}
							}
						}
					}
				}

				version.Types = append(version.Types, typeMetadata)
			}
		})
		if err != nil {
			return err
		}

		if len(version.Types) > 0 {
			version.ControllerPkg = filepath.Join(g.OutputPackage, "controllers", group.PackageName, versionName)
			version.Version = versionName
			version.VersionUpper = upperFirst(versionName)
			group.Versions = append(group.Versions, version)
		}
	}

	for _, group := range groups {
		metadata.Groups = append(metadata.Groups, *group)
	}

	if len(metadata.Groups) == 0 {
		return nil
	}

	osWriteFile := func(path string, data []byte) error {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
		formatted, err := imports.Process(path, data, nil)
		if err != nil {
			// If formatting fails, write the original data so we can debug it
			os.WriteFile(path, data, 0644)
			return fmt.Errorf("failed to format %s: %w", path, err)
		}
		return os.WriteFile(path, formatted, 0644)
	}

	for _, group := range metadata.Groups {
		groupDir := filepath.Join("pkg", "generated", "controllers", group.PackageName)

		// Generate group interface.go
		var buf bytes.Buffer
		data := struct {
			GroupMetadata
			Boilerplate string
		}{
			GroupMetadata: group,
			Boilerplate:   boilerplate,
		}
		if err := groupInterfaceTemplate.Execute(&buf, data); err != nil {
			return err
		}
		groupInterfacePath := filepath.Join(groupDir, "interface.go")
		if err := osWriteFile(groupInterfacePath, buf.Bytes()); err != nil {
			return err
		}

		// Generate factory.go
		buf.Reset()
		if err := factoryTemplate.Execute(&buf, data); err != nil {
			return err
		}
		factoryPath := filepath.Join(groupDir, "factory.go")
		if err := osWriteFile(factoryPath, buf.Bytes()); err != nil {
			return err
		}

		for _, version := range group.Versions {
			versionDir := filepath.Join(groupDir, version.Version)

			// Generate version interface.go
			buf.Reset()
			data := struct {
				VersionMetadata
				Boilerplate string
			}{
				VersionMetadata: version,
				Boilerplate:     boilerplate,
			}
			if err := versionInterfaceTemplate.Execute(&buf, data); err != nil {
				return err
			}
			versionInterfacePath := filepath.Join(versionDir, "interface.go")
			if err := osWriteFile(versionInterfacePath, buf.Bytes()); err != nil {
				return err
			}

			for _, t := range version.Types {
				// Generate type.go
				buf.Reset()
				data := struct {
					TypeMetadata
					Boilerplate string
					TypesPkg    string
				}{
					TypeMetadata: t,
					Boilerplate:  boilerplate,
					TypesPkg:     version.TypesPkg,
				}
				if err := typeTemplate.Execute(&buf, data); err != nil {
					return err
				}
				fileName := strings.ToLower(t.Name) + ".go"
				typeFilePath := filepath.Join(versionDir, fileName)
				if err := osWriteFile(typeFilePath, buf.Bytes()); err != nil {
					return err
				}
			}
		}
	}

	if metadata.GenerateClientset {
		if err := g.runExternalGenerator("client-gen", metadata, "--clientset-name", "versioned", "--input-base", "", "--clientset-only=false"); err != nil {
			return err
		}
	}
	if metadata.GenerateListers {
		if err := g.runExternalGenerator("lister-gen", metadata); err != nil {
			return err
		}
	}
	if metadata.GenerateInformers {
		if err := g.runExternalGenerator("informer-gen", metadata, "--versioned-clientset-package", filepath.Join(g.OutputPackage, "clientset/versioned"), "--listers-package", filepath.Join(g.OutputPackage, "listers")); err != nil {
			return err
		}
	}

	return nil
}

func (g *WranglerGenerator) runExternalGenerator(name string, metadata *GenerationMetadata, extraArgs ...string) error {
	fmt.Printf("Invoking external generator: %s\n", name)
	// Base command
	args := []string{"run", "k8s.io/code-generator/cmd/" + name}
	
	var inputPkgs []string
	for _, group := range metadata.Groups {
		for _, version := range group.Versions {
			// For newer k8s generators, we pass the package path as positional argument
			inputPkgs = append(inputPkgs, version.TypesPkg)
		}
	}

	outputBase := "pkg/generated"
	outputPkg := g.OutputPackage + "/" + name + "s"
	var cmdArgs []string
	if strings.HasSuffix(name, "-gen") {
		stem := strings.TrimSuffix(name, "-gen")
		outputPkg = g.OutputPackage + "/" + stem + "s"
		cmdArgs = []string{
			"--output-pkg", outputPkg,
			"--output-dir", filepath.Join(outputBase, stem+"s"),
		}
	}
	if name == "client-gen" {
		outputPkg = g.OutputPackage + "/clientset"
		cmdArgs = []string{
			"--output-pkg", outputPkg,
			"--output-dir", filepath.Join(outputBase, "clientset"),
		}
	}

	if g.Boilerplate != "" {
		cmdArgs = append(cmdArgs, "--go-header-file", g.Boilerplate)
	}
	cmdArgs = append(cmdArgs, extraArgs...)
	cmdArgs = append(cmdArgs, inputPkgs...)

	fullArgs := append(args, cmdArgs...)

	cmd := exec.Command("/usr/local/go/bin/go", fullArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	fmt.Printf("Running: go %s\n", strings.Join(fullArgs, " "))
	return cmd.Run()
}

func (g *WranglerGenerator) Help() *markers.Definition {
	return markers.Must(markers.MakeDefinition("wrangler", markers.DescribesPackage, WranglerGenerator{}))
}
