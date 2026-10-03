# anatoliacore_security_group

Creates an OPNsense-enforced Edge Firewall policy. `rules_json` is a JSON array;
changes replace the policy atomically so partially applied rule sets are not
left behind.

```hcl
resource "anatoliacore_security_group" "web" {
  vdc_id = anatoliacore_vdc.main.id
  name   = "public-web"
  rules_json = jsonencode([{
    direction   = "ingress"
    protocol    = "tcp"
    port_start  = 443
    port_end    = 443
    source_cidr = "0.0.0.0/0"
    action      = "allow"
  }])
}
```

This filters routed/Internet traffic at the VDC edge; it is not an L2 firewall
between guests on the same port group.
