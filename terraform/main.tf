terraform {
  required_providers {
    digitalocean = {
      source  = "digitalocean/digitalocean"
      version = "~> 2.0"
    }
  }
}

provider "digitalocean" {
  token = var.do_token
}

resource "digitalocean_droplet" "redirect" {
  name     = "redirect-name"
  region   = var.region
  size     = var.droplet_size
  image    = "ubuntu-22-04-x64"
  ssh_keys = [var.ssh_key_fingerprint]

  # Runs on first boot: mounts the cert volume and creates the service user.
  user_data = <<-EOF
    #!/bin/bash
    set -euo pipefail

    # Create service user
    useradd --system --no-create-home --shell /usr/sbin/nologin redirect

    # Mount the cert volume (pre-formatted as ext4 by Terraform)
    mkdir -p /mnt/certs
    DEVICE="/dev/disk/by-id/scsi-0DO_Volume_redirect-certs"
    for i in $(seq 1 30); do
      [ -e "$DEVICE" ] && break
      sleep 1
    done
    if [ -e "$DEVICE" ]; then
      echo "$DEVICE /mnt/certs ext4 defaults,nofail,discard 0 2" >> /etc/fstab
      mount -a
      chown redirect:redirect /mnt/certs
    fi
  EOF
}

resource "digitalocean_volume" "certs" {
  region                  = var.region
  name                    = "redirect-certs"
  size                    = 1
  initial_filesystem_type = "ext4"
  description             = "TLS cert cache for redirect-name"
}

resource "digitalocean_volume_attachment" "certs" {
  droplet_id = digitalocean_droplet.redirect.id
  volume_id  = digitalocean_volume.certs.id
}

resource "digitalocean_firewall" "redirect" {
  name        = "redirect-name"
  droplet_ids = [digitalocean_droplet.redirect.id]

  inbound_rule {
    protocol         = "tcp"
    port_range       = "80"
    source_addresses = ["0.0.0.0/0", "::/0"]
  }

  inbound_rule {
    protocol         = "tcp"
    port_range       = "443"
    source_addresses = ["0.0.0.0/0", "::/0"]
  }

  inbound_rule {
    protocol         = "tcp"
    port_range       = "22"
    source_addresses = ["0.0.0.0/0", "::/0"]
  }

  outbound_rule {
    protocol              = "tcp"
    port_range            = "1-65535"
    destination_addresses = ["0.0.0.0/0", "::/0"]
  }

  outbound_rule {
    protocol              = "udp"
    port_range            = "1-65535"
    destination_addresses = ["0.0.0.0/0", "::/0"]
  }

  outbound_rule {
    protocol              = "icmp"
    destination_addresses = ["0.0.0.0/0", "::/0"]
  }
}

# Phase 3 cutover: uncomment once HTTP is validated on the ephemeral IP.
# This reassigns the reserved IP from the old droplet to the new one (~5s outage).
#
# resource "digitalocean_reserved_ip_assignment" "redirect" {
#   ip_address = var.reserved_ip
#   droplet_id = digitalocean_droplet.redirect.id
# }
