---
page_title: "anatoliacore_vdc Resource - AnatoliaCore"
description: |-
  Manages an AnatoliaCore virtual data center.
---

# anatoliacore_vdc

Manages an AnatoliaCore virtual data center and its resource limits.

```terraform
resource "anatoliacore_vdc" "production" {
  name           = "production"
  cpu_limit_mhz  = 16000
  ram_limit_mb   = 65536
  disk_limit_gb  = 1000
  public_ip_count = 2
}
```

## Schema

### Required

- `name` (String) VDC name: 1–63 characters, starting with an alphanumeric character.
- `cpu_limit_mhz` (Number) CPU limit from 1000 through 1000000 MHz.
- `ram_limit_mb` (Number) Memory limit from 1024 through 4194304 MiB, in 256 MiB increments.
- `disk_limit_gb` (Number) Disk limit from 10 through 1048576 GiB.

### Optional

- `public_ip_count` (Number) Public IPv4 address count for the OPNsense edge, from 1 through 5 (default: 1). Changing it replaces the VDC.

### Read-Only

- `id` (String) VDC identifier.
- `status` (String) VDC lifecycle status.
- `hourly_price` (Number) Current hourly price.
- `public_ips` (List of String) Allocated public IPv4 addresses; the first address is the primary WAN address.
