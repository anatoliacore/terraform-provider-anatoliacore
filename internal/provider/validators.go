package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

type multipleOfInt64 int64

var _ validator.Int64 = multipleOfInt64(1)

func (v multipleOfInt64) Description(_ context.Context) string {
	return fmt.Sprintf("value must be a multiple of %d", v)
}

func (v multipleOfInt64) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v multipleOfInt64) ValidateInt64(
	_ context.Context,
	req validator.Int64Request,
	resp *validator.Int64Response,
) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	if v <= 0 || req.ConfigValue.ValueInt64()%int64(v) != 0 {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid numeric value",
			fmt.Sprintf("Value must be a multiple of %d.", v),
		)
	}
}
