package utils

func GetDeviceType(device_type string) int32 {
	switch device_type {
	case "desktop":
		return 1
	case "mobile":
		return 2
	case "tablet":
		return 3
	case "bot":
		return 4
	default:
		return 5
	}
}
