package controllergen

import (
	"path/filepath"
	"reflect"
	"strings"

	"golang.org/x/tools/imports"
	"sigs.k8s.io/controller-tools/pkg/genall"
)

func (g *WranglerGenerator) collectManualMetadata(ctx *genall.GenerationContext, groups map[string]*GroupMetadata, metadata *GenerationMetadata) error {
	for groupName, manualGroup := range g.ManualGroups {
		metadata.GenerateClientset = metadata.GenerateClientset || manualGroup.GenerateClients
		metadata.GenerateListers = metadata.GenerateListers || manualGroup.GenerateListers
		metadata.GenerateInformers = metadata.GenerateInformers || manualGroup.GenerateInformers

		for _, obj := range manualGroup.Types {
			var (
				pkgName string
				typeName string
			)

			t := reflect.TypeOf(obj)
			if t.Kind() == reflect.Ptr {
				t = t.Elem()
			}
			pkgName = imports.VendorlessPath(t.PkgPath())
			typeName = t.Name()
			
			versionName := versionFromPackage(pkgName)

			group, ok := groups[groupName]
			if !ok {
				pkgName := manualGroup.OutputControllerPackageName
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
					CustomPackageName: manualGroup.OutputControllerPackageName,
				}
				groups[groupName] = group
			}

			var version *VersionMetadata
			for i := range group.Versions {
				if group.Versions[i].Version == versionName {
					version = &group.Versions[i]
					break
				}
			}

			if version == nil {
				versionPkg := filepath.Join(g.OutputPackage, "controllers", group.PackageName, versionName)
				group.Versions = append(group.Versions, VersionMetadata{
					Version:       versionName,
					VersionUpper:  upperFirst(versionName),
					TypesPkg:      pkgName,
					ControllerPkg: versionPkg,
				})
				version = &group.Versions[len(group.Versions)-1]
			}

			typeMetadata := TypeMetadata{
				Name:        typeName,
				LowerName:   lowerFirst(typeName),
				Plural:      pluralize(typeName),
				PluralLower: strings.ToLower(pluralize(typeName)),
				Group:       groupName,
				Version:     versionName,
			}

			typeMetadata.Namespaced = true 
			
			version.Types = append(version.Types, typeMetadata)
		}
	}
	return nil
}

func versionFromPackage(pkg string) string {
	parts := strings.Split(pkg, "/")
	return parts[len(parts)-1]
}
