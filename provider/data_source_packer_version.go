package provider

import (
	"context"
	"os"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/toowoxx/go-lib-userspace-common/cmds"
)

type dataSourceVersionType struct {
	Version string `tfsdk:"version"`
}

func (r dataSourceVersionType) GetSchema(_ context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		Attributes: map[string]tfsdk.Attribute{
			"version": {
				Description: "Version of installed JuiceFS",
				Computed:    true,
				Type:        types.StringType,
			},
		},
	}, nil
}

func (r dataSourceVersionType) NewDataSource(ctx context.Context, p tfsdk.Provider) (tfsdk.DataSource, diag.Diagnostics) {
	return dataSourceVersion{
		p: *(p.(*provider)),
	}, nil
}

type dataSourceVersion struct {
	p provider
}

func (r dataSourceVersion) Read(ctx context.Context, req tfsdk.ReadDataSourceRequest, resp *tfsdk.ReadDataSourceResponse) {
	resourceState := dataSourceVersionType{}
	exe, _ := os.Executable()
	output, err := cmds.RunCommandWithEnvReturnOutput(
		exe,
		map[string]string{TJPRunJFS: "true"},
		"--version")
	if err != nil {
		resp.Diagnostics.AddError("Failed to run juicefs", err.Error())
		return
	}

	if len(output) == 0 {
		resp.Diagnostics.AddError("Unexpected output", "JuiceFS did not output anything")
		return
	}

	resourceState.Version = strings.TrimSpace(strings.TrimPrefix(
		strings.TrimSpace(strings.TrimPrefix(string(output), "juicefs")), "version"))

	diags := resp.State.Set(ctx, &resourceState)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}
