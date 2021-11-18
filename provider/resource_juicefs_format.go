package provider

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/pkg/errors"
	"github.com/toowoxx/go-lib-userspace-common/cmds"
	"gitlab.com/xdevs23/go-collections"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

type resourceFormat struct {
	ID               types.String      `tfsdk:"id"`
	AdditionalParams []string          `tfsdk:"additional_params"`
	Environment      map[string]string `tfsdk:"environment"`
	Triggers         map[string]string `tfsdk:"triggers"`
	Force            types.Bool        `tfsdk:"force"`
	Storage          types.String      `tfsdk:"storage"`
	Bucket           types.String      `tfsdk:"bucket"`
	MetadataURI      types.String      `tfsdk:"metadata_uri"`
	StorageName      types.String      `tfsdk:"storage_name"`

	AzureStorageEndpointSuffixFix types.Bool `tfsdk:"azure_storage_endpoint_suffix_fix"`
}

var validStorages = []string{
	"file", "mem", "redis", "s3", "sftp", "wasb", "webdav",
}

type resourceFormatStorageValidator struct{}

func (r resourceFormat) GetSchema(_ context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		Attributes: map[string]tfsdk.Attribute{
			"id": {
				Type:     types.StringType,
				Computed: true,
			},
			"additional_params": {
				Description: "Additional parameters to pass to the JuiceFS command",
				Type:        types.SetType{ElemType: types.StringType},
				Optional:    true,
			},
			"force": {
				Description: "Force overwriting existing images",
				Type:        types.BoolType,
				Optional:    true,
			},
			"environment": {
				Description: "Environment variables",
				Type:        types.MapType{ElemType: types.StringType},
				Optional:    true,
			},
			"triggers": {
				Description: "Values that, when changed, trigger an update of this resource",
				Type:        types.MapType{ElemType: types.StringType},
				Optional:    true,
			},
			"storage": {
				Description: "Storage to use (--storage parameter). Supported are: " +
					strings.Join(validStorages, ", "),
				Type:       types.StringType,
				Validators: []tfsdk.AttributeValidator{resourceFormatStorageValidator{}},
				Required:   true,
			},
			"bucket": {
				Description: "The bucket URL to use",
				Type:        types.StringType,
				Optional:    true,
			},
			"azure_storage_endpoint_suffix_fix": {
				Description: "It may be necessary to use '*.core.windows.net' instead of '*.blob.core.windows.net'. " +
					"This parameter does just that.",
				Type:     types.BoolType,
				Optional: true,
			},
			"metadata_uri": {
				Description: "Metadata engine to use (for example redis://localhost/1)",
				Type:        types.StringType,
				Required:    true,
			},
			"storage_name": {
				Description: "Storage name",
				Type:        types.StringType,
				Required:    true,
			},
		},
	}, nil
}

func (r resourceFormatStorageValidator) Description(ctx context.Context) string {
	return "Storage needs to be supported."
}

func (r resourceFormatStorageValidator) MarkdownDescription(ctx context.Context) string {
	return "Storage needs to be supported."
}

func (r resourceFormatStorageValidator) Validate(ctx context.Context, request tfsdk.ValidateAttributeRequest, response *tfsdk.ValidateAttributeResponse) {
	attr, err := request.AttributeConfig.ToTerraformValue(ctx)
	if err != nil {
		response.Diagnostics.AddError("validation failed", err.Error())
		return
	}
	if !collections.Include(validStorages, attr.(string)) {
		response.Diagnostics.AddError("validation failed", fmt.Sprintf("storage %s is not supported", attr))
	}
}

func (r resourceFormat) NewResource(_ context.Context, p tfsdk.Provider) (tfsdk.Resource, diag.Diagnostics) {
	return resourceImage{
		p: *(p.(*provider)),
	}, nil
}

type resourceImage struct {
	p provider
}

func (r resourceImage) ImportState(ctx context.Context, req tfsdk.ImportResourceStateRequest, resp *tfsdk.ImportResourceStateResponse) {
	tfsdk.ResourceImportStatePassthroughID(ctx, tftypes.NewAttributePath().WithAttributeName("id"), req, resp)
}

func (r resourceImage) juiceFSFormat(resourceState *resourceFormat) error {
	envVars := map[string]string{}
	for key, value := range resourceState.Environment {
		envVars[key] = value
	}
	envVars[TJPRunJFS] = "true"

	/*
	   export AZURE_STORAGE_CONNECTION_STRING='${var.azure_blob_connection_string}'
	   juicefs format \
	     --storage wasb \
	     --bucket '${replace(var.blob_endpoint, ".blob.core.windows.net", ".core.windows.net")}${var.storage_name}' \
	     redis://${var.redis_host}/${var.redis_db_number} \
	     ${var.storage_name}
	*/

	bucket := resourceState.Bucket.Value
	if resourceState.AzureStorageEndpointSuffixFix.Value {
		bucket = strings.Replace(".blob.core.windows.net", bucket, ".core.windows.net", 1)
	}

	params := []string{
		"format",
		"--storage", resourceState.Storage.Value,
	}
	if !resourceState.Bucket.Unknown && len(resourceState.Bucket.Value) > 0 {
		params = append(params, "--bucket", bucket)
	}
	if resourceState.Force.Value {
		params = append(params, "--force")
	}
	params = append(params, resourceState.AdditionalParams...)
	params = append(params,
		resourceState.MetadataURI.Value,
		resourceState.StorageName.Value,
	)

	exe, _ := os.Executable()
	output, err := cmds.RunCommandWithEnvReturnOutput(exe, envVars, params...)

	if err != nil {
		return errors.Wrap(err, "could not run juicefs command; output: "+string(output))
	}

	return nil
}

func (r resourceImage) updateState(resourceState *resourceFormat) error {
	if resourceState.ID.Unknown {
		resourceState.ID = types.String{Value: uuid.Must(uuid.NewRandom()).String()}
	}

	return nil
}

func (r resourceImage) Create(ctx context.Context, req tfsdk.CreateResourceRequest, resp *tfsdk.CreateResourceResponse) {
	resourceState := resourceFormat{}
	diags := req.Config.Get(ctx, &resourceState)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.juiceFSFormat(&resourceState)
	if err != nil {
		resp.Diagnostics.AddError("Failed to run juicefs init", err.Error())
		return
	}
	err = r.updateState(&resourceState)
	if err != nil {
		resp.Diagnostics.AddError("Failed to run juicefs", err.Error())
		return
	}

	diags = resp.State.Set(ctx, &resourceState)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r resourceImage) Read(ctx context.Context, req tfsdk.ReadResourceRequest, resp *tfsdk.ReadResourceResponse) {
	var state resourceFormat
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r resourceImage) Update(ctx context.Context, req tfsdk.UpdateResourceRequest, resp *tfsdk.UpdateResourceResponse) {
	var plan resourceFormat
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state resourceFormat
	diags = req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.juiceFSFormat(&plan)
	if err != nil {
		resp.Diagnostics.AddError("Failed to run juicefs init", err.Error())
		return
	}
	err = r.updateState(&plan)
	if err != nil {
		resp.Diagnostics.AddError("Failed to run juicefs", err.Error())
		return
	}

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r resourceImage) Delete(ctx context.Context, req tfsdk.DeleteResourceRequest, resp *tfsdk.DeleteResourceResponse) {
	var state resourceFormat
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.State.RemoveResource(ctx)
}
