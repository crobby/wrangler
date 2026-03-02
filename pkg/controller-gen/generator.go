package controllergen

import (
	"bytes"
	"go/ast"
	"go/types"
	"os"
	"path/filepath"
	"strings"

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
		group, ok := groups[groupName]
		if !ok {
			group = &GroupMetadata{
				Name:        groupName,
				UpperName:   upperFirst(strings.Split(groupName, ".")[0]),
				PackageName: strings.ReplaceAll(strings.Split(groupName, ".")[0], "-", ""),
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
			group.Versions = append(group.Versions, version)
		}
	}

	for _, group := range groups {
		metadata.Groups = append(metadata.Groups, *group)
	}

	if len(metadata.Groups) == 0 {
		return nil
	}

	for _, group := range metadata.Groups {
		groupDir := filepath.Join("pkg", "generated", "controllers", group.PackageName)
		if err := os.MkdirAll(groupDir, 0755); err != nil {
			return err
		}

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
		if err := os.WriteFile(filepath.Join(groupDir, "interface.go"), buf.Bytes(), 0644); err != nil {
			return err
		}

		for _, version := range group.Versions {
			versionDir := filepath.Join(groupDir, version.Version)
			if err := os.MkdirAll(versionDir, 0755); err != nil {
				return err
			}

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
			if err := os.WriteFile(filepath.Join(versionDir, "interface.go"), buf.Bytes(), 0644); err != nil {
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
				if err := os.WriteFile(filepath.Join(versionDir, fileName), buf.Bytes(), 0644); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (g *WranglerGenerator) Help() *markers.Definition {
	return markers.Must(markers.MakeDefinition("wrangler", markers.DescribesPackage, WranglerGenerator{}))
}
