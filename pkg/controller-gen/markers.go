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
)

// Generate is the type that corresponds to the +wrangler:generate marker.
type Generate struct {
}

// Group is the type that corresponds to the +wrangler:group marker.
type Group struct {
	Name string `marker:"name"`
}
