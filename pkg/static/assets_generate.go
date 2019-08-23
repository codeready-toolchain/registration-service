// +build ignore

package main

import (
	"log"

	"github.com/codeready-toolchain/registration-service/pkg/static"
	"github.com/shurcooL/vfsgen"
)

func main() {
	err := vfsgen.Generate(static.Assets, vfsgen.Options{
		PackageName:  "static",
		BuildTags:    "!dev",
		VariableName: "Assets",
		Filename:     "../static/generated_assets.go",
	})
	if err != nil {
		log.Fatalln(err)
	}
}
