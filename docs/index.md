---
page_title: "AnatoliaCore Provider"
description: |-
  The AnatoliaCore provider manages customer cloud infrastructure through the AnatoliaCore Public API.
---

# AnatoliaCore Provider

Use the AnatoliaCore provider to manage instances and VDCs with Terraform.

```terraform
terraform {
  required_providers {
    anatoliacore = {
      source  = "anatoliacore/anatoliacore"
      version = "~> 1.0"
    }
  }
}

provider "anatoliacore" {}
```

Set `ANATOLIACORE_API_KEY` outside Terraform configuration. The provider only
accepts credential-free HTTPS API endpoints and sends a unique idempotency key
for every mutation.

## Schema

### Optional

- `api_key` (String, Sensitive) AnatoliaCore API key. Defaults to
  `ANATOLIACORE_API_KEY`.
- `base_url` (String) Public API base URL. Defaults to
  `ANATOLIACORE_BASE_URL` or `https://console.anatoliacore.com/api/public/v1`.
