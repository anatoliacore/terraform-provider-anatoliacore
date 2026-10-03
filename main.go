package main

import (
	"context"
	"flag"
	"log"

	"github.com/anatoliacore/terraform-provider-anatoliacore/internal/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
)

var version = "dev"
var commit = "none"

func main() {
	var debug bool
	flag.BoolVar(&debug, "debug", false, "run the provider with debugger support")
	flag.Parse()
	err := providerserver.Serve(context.Background(), provider.New(version), providerserver.ServeOpts{
		Address: "registry.terraform.io/anatoliacore/anatoliacore",
		Debug:   debug,
	})
	if err != nil {
		log.Fatal(err)
	}
}
