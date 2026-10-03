# anatoliacore_floating_ip

Allocates a public IP from a VDC and optionally associates it with an instance
in the same VDC.

```hcl
resource "anatoliacore_floating_ip" "web" {
  vdc_id      = anatoliacore_vdc.main.id
  instance_id = anatoliacore_instance.web.id
}
```

Import with `terraform import anatoliacore_floating_ip.web <floating-ip-id>`.
