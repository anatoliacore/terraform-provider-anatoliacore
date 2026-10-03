# anatoliacore_volume

Creates an independently billed block volume. `size_gb` can only grow. Setting
`instance_id` attaches the volume; changing or removing it safely detaches and
reattaches the volume.

```hcl
resource "anatoliacore_volume" "data" {
  name        = "database-data"
  size_gb     = 100
  instance_id = anatoliacore_instance.web.id
}
```

Import with `terraform import anatoliacore_volume.data <volume-id>`.
