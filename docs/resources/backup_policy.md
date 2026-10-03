# anatoliacore_backup_policy

Creates a self-service backup schedule for one or more instances.

```hcl
resource "anatoliacore_backup_policy" "daily" {
  name             = "daily-production"
  instance_ids     = [anatoliacore_instance.web.id]
  schedule_type    = "daily"
  start_time       = "22:00"
  timezone         = "Europe/Istanbul"
  retention_days   = 14
  schedule_enabled = true
}
```

Import with `terraform import anatoliacore_backup_policy.daily <policy-id>`.
