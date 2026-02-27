output "reserved_ip" {
  description = "Reserved IP — point DNS A records here"
  value       = digitalocean_reserved_ip.redirect.ip_address
}

output "droplet_ip" {
  description = "Droplet ephemeral IP (use reserved_ip for DNS)"
  value       = digitalocean_droplet.redirect.ipv4_address
}

output "droplet_id" {
  description = "Droplet ID"
  value       = digitalocean_droplet.redirect.id
}
