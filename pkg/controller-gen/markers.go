package controllergen

import (
	"sigs.k8s.io/controller-tools/pkg/markers"
)

var (
	// GenerateMarker indicates that a struct should have Wrangler controllers generated for it.
	// Marker: +wrangler:generate
	GenerateMarker = markers.Must(markers.MakeDefinition("wrangler:generate", markers.DescribesType, Generate{}))

	// GroupMarker defines the Wrangler group for a package.
	// Marker: +wrangler:group:name=<group.name.com>
	GroupMarker = markers.Must(markers.MakeDefinition("wrangler:group", markers.DescribesPackage, Group{}))

	// ClientsetMarker indicates that a standard Kubernetes clientset should be generated for this package.
	// Marker: +wrangler:generate:clientset
	ClientsetMarker = markers.Must(markers.MakeDefinition("wrangler:generate:clientset", markers.DescribesPackage, Clientset{}))

	// ListersMarker indicates that standard Kubernetes listers should be generated for this package.
	// Marker: +wrangler:generate:listers
	ListersMarker = markers.Must(markers.MakeDefinition("wrangler:generate:listers", markers.DescribesPackage, Listers{}))

	// InformersMarker indicates that standard Kubernetes informers should be generated for this package.
	// Marker: +wrangler:generate:informers
	InformersMarker = markers.Must(markers.MakeDefinition("wrangler:generate:informers", markers.DescribesPackage, Informers{}))
)

// Generate is the type that corresponds to the +wrangler:generate marker.
type Generate struct {
}

// Group is the type that corresponds to the +wrangler:group marker.
type Group struct {
	Name        string `marker:"name"`
	PackageName string `marker:"packageName,optional"`
}

// Clientset is the type that corresponds to the +wrangler:generate:clientset marker.
type Clientset struct {
}

// Listers is the type that corresponds to the +wrangler:generate:listers marker.
type Listers struct {
}

// Informers is the type that corresponds to the +wrangler:generate:informers marker.
type Informers struct {
}
