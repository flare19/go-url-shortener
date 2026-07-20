resource "azurerm_storage_account" "frontend" {
  name                     = "${var.project_name}frontend" # must be globally unique, lowercase, no hyphens
  resource_group_name      = azurerm_resource_group.main.name
  location                 = azurerm_resource_group.main.location
  account_tier             = "Standard"
  account_replication_type = "LRS"

  static_website {
    index_document = "index.html"
  }
}