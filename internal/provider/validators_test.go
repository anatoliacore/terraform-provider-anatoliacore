package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestMultipleOfInt64(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		value     types.Int64
		wantError bool
	}{
		{name: "valid", value: types.Int64Value(1024)},
		{name: "invalid", value: types.Int64Value(1000), wantError: true},
		{name: "unknown", value: types.Int64Unknown()},
		{name: "null", value: types.Int64Null()},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			request := validator.Int64Request{Path: path.Root("ram_mb"), ConfigValue: test.value}
			response := &validator.Int64Response{}
			multipleOfInt64(256).ValidateInt64(context.Background(), request, response)
			if response.Diagnostics.HasError() != test.wantError {
				t.Fatalf("HasError() = %v, want %v", response.Diagnostics.HasError(), test.wantError)
			}
		})
	}
}
