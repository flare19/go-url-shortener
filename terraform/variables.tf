variable "location" {
  description = "Azure region for all resources"
  type        = string
  default     = "centralindia"
}

variable "project_name" {
  description = "Short name used as a prefix for resource naming"
  type        = string
  default     = "urlshort"
}

variable "admin_username" {
  type    = string
  default = "tejas"
}

variable "ssh_public_key_path" {
  type    = string
  default = "~/.ssh/id_rsa_azure.pub"
}