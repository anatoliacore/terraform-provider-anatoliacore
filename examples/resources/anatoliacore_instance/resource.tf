resource "anatoliacore_instance" "example" {
  name        = "terraform-example"
  cpu_cores   = 2
  ram_mb      = 4096
  disk_gb     = 40
  os_template = "ubuntu-24-04"
}
