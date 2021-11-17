package main

//go:generate go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@latest

import (
	"context"
	"log"
	"os"

	"terraform-provider-juicefs/provider"

	"github.com/hashicorp/terraform-plugin-framework/tfsdk"

	juicefs_cmd "github.com/juicedata/juicefs/cmd"
)

func main() {
	if os.Getenv(provider.TJPRunJFS) == "true" {
		os.Exit(juicefs_cmd.Main(os.Args[1:]))
	} else {
		if err := tfsdk.Serve(context.Background(), provider.New, tfsdk.ServeOpts{
			Name: "juicefs",
		}); err != nil {
			log.Fatal(err)
		}
	}
}
