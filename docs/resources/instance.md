---
page_title: "anatoliacore_instance Resource - AnatoliaCore"
description: |-
  Manages an AnatoliaCore instance.
---

# anatoliacore_instance

Manages an AnatoliaCore virtual machine. CPU, RAM, and disk changes are applied
through the API resize operation. Changing the name, OS template, VDC, SSH key,
or cloud-init user data replaces the instance.

```terraform
resource "anatoliacore_instance" "web" {
  name        = "web-01"
  cpu_cores   = 2
  ram_mb      = 4096
  disk_gb     = 40
  os_template = "ubuntu-24-04"
}
```

## Schema

### Required

- `name` (String) Instance name: 1–63 characters, starting with an alphanumeric character.
- `cpu_cores` (Number) CPU cores from 1 through 64.
- `ram_mb` (Number) Memory from 512 through 262144 MiB, in 256 MiB increments.
- `disk_gb` (Number) Disk size from 10 through 4096 GiB.
- `os_template` (String) Operating system template identifier.

### Optional

- `vdc_id` (String) Target VDC identifier.
- `ssh_key_id` (String) SSH key identifier injected at provisioning time.
- `user_data` (String, Sensitive) cloud-init user data.

### Read-Only

- `id` (String) Instance identifier.
- `ip_address` (String) Assigned primary IP address.
- `status` (String) Instance lifecycle status.
- `power_state` (String) Hypervisor power state.
- `hourly_price` (Number) Current hourly price.
- `monthly_price` (Number) Current monthly price.
