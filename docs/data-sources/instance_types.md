---
page_title: "anatoliacore_instance_types Data Source - AnatoliaCore"
description: |-
  Lists the AnatoliaCore instance type catalog.
---

# anatoliacore_instance_types

Lists the currently available instance type catalog.

```terraform
data "anatoliacore_instance_types" "available" {}
```

## Read-Only

- `id` (String) Always `catalog`.
- `types` (List of Object) Catalog records with `id`, `name`, `cpu_cores`,
  `ram_mb`, `disk_gb`, `description`, and `price_per_hour`.
