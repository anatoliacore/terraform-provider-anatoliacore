# AnatoliaCore Terraform Provider

The provider uses Terraform Plugin Protocol 6 and manages AnatoliaCore
instances, VDCs, volumes, floating IPs, edge security groups, and backup
policies. It also publishes the instance type catalog as a data source.

```hcl
terraform {
  required_providers {
    anatoliacore = {
      source  = "anatoliacore/anatoliacore"
      version = "~> 1.0"
    }
  }
}

provider "anatoliacore" {}

resource "anatoliacore_instance" "web" {
  name        = "web-01"
  cpu_cores   = 2
  ram_mb      = 4096
  disk_gb     = 40
  os_template = "ubuntu-24-04"
}
```

Set `ANATOLIACORE_API_KEY` in the process environment. Do not place API keys
in `.tf` files or commit them into Terraform state-adjacent configuration.

Every mutation uses a unique idempotency key internally. The provider follows
the platform operation contract and treats redirects as errors so credentials
cannot be forwarded to an unexpected host.
