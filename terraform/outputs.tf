output "writer_public_ip" {
  value = azurerm_public_ip.writer.ip_address
}

output "redirector_public_ip" {
  value = azurerm_public_ip.redirector.ip_address
}

output "frontend_static_url" {
  value = azurerm_storage_account.frontend.primary_web_endpoint
}