variable "do_token" {
  description = "DigitalOcean API token — set via TF_VAR_do_token env var, never in files"
  type        = string
  sensitive   = true
}

variable "region" {
  description = "DigitalOcean region slug"
  type        = string
  default     = "nyc1"
}

variable "droplet_size" {
  description = "DigitalOcean droplet size slug"
  type        = string
  default     = "s-1vcpu-1gb"
}

variable "ssh_key_fingerprint" {
  description = "Fingerprint of deploy SSH key — visible in DO control panel → Settings → Security"
  type        = string
}
