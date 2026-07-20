resource "azurerm_linux_virtual_machine" "writer" {
  name                = "${var.project_name}-writer"
  location            = azurerm_resource_group.main.location
  resource_group_name = azurerm_resource_group.main.name
  size                = "Standard_B2ats_v2"
  admin_username      = var.admin_username
  network_interface_ids = [azurerm_network_interface.writer.id]

  disable_password_authentication = true
  admin_ssh_key {
    username   = var.admin_username
    public_key = file(var.ssh_public_key_path)
  }

  os_disk {
    caching              = "ReadWrite"
    storage_account_type = "Premium_LRS" # pinned: matches free-tier P6 allowance
    disk_size_gb          = 64            # pinned: matches free-tier allowance
  }

  source_image_reference {
    publisher = "Canonical"
    offer     = "0001-com-ubuntu-server-jammy"
    sku       = "22_04-lts-gen2"
    version   = "latest"
  }
}

resource "azurerm_linux_virtual_machine" "redirector" {
  name                = "${var.project_name}-redirector"
  location            = azurerm_resource_group.main.location
  resource_group_name = azurerm_resource_group.main.name
  size                = "Standard_B2ats_v2"
  admin_username      = var.admin_username
  network_interface_ids = [azurerm_network_interface.redirector.id]

  disable_password_authentication = true
  admin_ssh_key {
    username   = var.admin_username
    public_key = file(var.ssh_public_key_path)
  }

  os_disk {
    caching              = "ReadWrite"
    storage_account_type = "Premium_LRS" # pinned: matches free-tier P6 allowance
    disk_size_gb          = 64            # pinned: matches free-tier allowance
  }

  source_image_reference {
    publisher = "Canonical"
    offer     = "0001-com-ubuntu-server-jammy"
    sku       = "22_04-lts-gen2"
    version   = "latest"
  }
}