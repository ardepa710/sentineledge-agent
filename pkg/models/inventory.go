package models

type InventoryCPU struct {
	Name          string `json:"name"`
	NumberOfCores int    `json:"number_of_cores"`
}

type InventoryRAM struct {
	TotalPhysicalMemoryGB float64 `json:"total_physical_memory_gb"`
}

type InventoryBIOS struct {
	SMBIOSBIOSVersion string `json:"smbios_bios_version"`
	Manufacturer      string `json:"manufacturer"`
}

type InventoryComputer struct {
	Manufacturer string `json:"manufacturer"`
	Model        string `json:"model"`
}

type InventorySerial struct {
	SerialNumber string `json:"serial_number"`
}

type InventoryDisk struct {
	DeviceID string  `json:"device_id"`
	SizeGB   float64 `json:"size_gb"`
	FreeGB   float64 `json:"free_gb"`
}

type InventoryNIC struct {
	Description string   `json:"description"`
	MACAddress  string   `json:"mac_address"`
	IPAddresses []string `json:"ip_addresses"`
}

type InventorySoftware struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Publisher   string `json:"publisher"`
	InstallDate string `json:"install_date"`
}

type Inventory struct {
	AgentID  string              `json:"agent_id"`
	Hostname string              `json:"hostname"`
	OS       string              `json:"os"`
	CPU      InventoryCPU        `json:"cpu"`
	RAM      InventoryRAM        `json:"ram"`
	BIOS     InventoryBIOS       `json:"bios"`
	Computer InventoryComputer   `json:"computer"`
	Serial   InventorySerial     `json:"serial"`
	Disks    []InventoryDisk     `json:"disks"`
	NICs     []InventoryNIC      `json:"nics"`
	Software []InventorySoftware `json:"software"`
}
