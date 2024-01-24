//go:build mage
// +build mage

package main

import (
	// mage:import
	build "github.com/grafana/grafana-plugin-sdk-go/build"
	"github.com/magefile/mage/mg"
)

// Build is copied from Grafana's SDK (build package), because the
// Arrow dependency doesn't support Linux ARM in the build matrix. I've removed
// it from the listing here.
func Build() {
	b := build.Build{}
	mg.Deps(b.Linux, b.Windows, b.Darwin, b.DarwinARM64, b.LinuxARM64, b.LinuxARM, b.GenerateManifestFile)
}

// Default configures the default target.
var Default = Build
