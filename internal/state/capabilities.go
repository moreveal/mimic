package state

// Capabilities describes the selected machine and backend policy, not WebIDL
// exposure. A missing backend never fabricates a connected device or session.
type Capabilities struct {
	StorageQuotaBytes int64
	Devices           DeviceCapabilities
	Media             MediaCapabilities
	KeyboardLayout    map[string]string
}

type DeviceCapabilities struct {
	BluetoothAvailable bool
	Posture            string
}

type MediaCapabilities struct {
	RTP     RTPCatalog
	Formats MediaFormatCatalog
	// Kinds are the machine's installed device classes. Before permission,
	// enumeration discloses at most one redacted device per class.
	Kinds                []string
	SupportedConstraints []string
}
