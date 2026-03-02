package controllergen

type GenerationMetadata struct {
	Groups []GroupMetadata
}

type GroupMetadata struct {
	Name            string
	UpperName       string
	PackageName     string
	Versions        []VersionMetadata
	ControllerPkg   string
}

type VersionMetadata struct {
	Version       string
	VersionUpper  string
	Types         []TypeMetadata
	TypesPkg      string
	ControllerPkg string
}

type TypeMetadata struct {
	Name         string
	LowerName    string
	Plural       string
	PluralLower  string
	Namespaced   bool
	HasStatus    bool
	StatusType   string
	Group        string
	Version      string
}
