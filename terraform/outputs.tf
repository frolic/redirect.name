output "droplet_ip" {
  description = "New droplet public IP — use this for pre-cutover smoke testing"
  value       = digitalocean_droplet.redirect.ipv4_address
}

output "droplet_id" {
  description = "New droplet ID"
  value       = digitalocean_droplet.redirect.id
}
