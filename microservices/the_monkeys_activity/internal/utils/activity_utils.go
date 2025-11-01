package utils

import (
	"strings"

	activitypb "github.com/the-monkeys/the_monkeys/apis/serviceconn/gateway_activity/pb"
)

// DetectPlatformFromUserAgent analyzes user agent string to determine platform
func DetectPlatformFromUserAgent(userAgent string) activitypb.Platform {
	if userAgent == "" {
		return activitypb.Platform_PLATFORM_WEB
	}

	userAgentLower := strings.ToLower(userAgent)

	// Check for mobile devices
	if strings.Contains(userAgentLower, "mobile") ||
		strings.Contains(userAgentLower, "android") ||
		strings.Contains(userAgentLower, "iphone") ||
		strings.Contains(userAgentLower, "ipod") ||
		strings.Contains(userAgentLower, "blackberry") ||
		strings.Contains(userAgentLower, "windows phone") {
		return activitypb.Platform_PLATFORM_MOBILE
	}

	// Check for tablets
	if strings.Contains(userAgentLower, "tablet") ||
		strings.Contains(userAgentLower, "ipad") ||
		strings.Contains(userAgentLower, "kindle") ||
		strings.Contains(userAgentLower, "playbook") {
		return activitypb.Platform_PLATFORM_TABLET
	}

	// Check for desktop applications or API clients
	if strings.Contains(userAgentLower, "postman") ||
		strings.Contains(userAgentLower, "insomnia") ||
		strings.Contains(userAgentLower, "curl") ||
		strings.Contains(userAgentLower, "wget") ||
		strings.Contains(userAgentLower, "httpie") ||
		strings.Contains(userAgentLower, "api") {
		return activitypb.Platform_PLATFORM_API
	}

	// Check for desktop apps
	if strings.Contains(userAgentLower, "electron") ||
		strings.Contains(userAgentLower, "nwjs") ||
		strings.Contains(userAgentLower, "desktop") {
		return activitypb.Platform_PLATFORM_DESKTOP
	}

	// Default to web for browsers
	return activitypb.Platform_PLATFORM_WEB
}

// DetectDeviceTypeFromUserAgent analyzes user agent to determine device type
func DetectDeviceTypeFromUserAgent(userAgent string) activitypb.DeviceType {
	if userAgent == "" {
		return activitypb.DeviceType_DEVICE_TYPE_DESKTOP
	}

	userAgentLower := strings.ToLower(userAgent)

	// Check for mobile devices
	if strings.Contains(userAgentLower, "mobile") ||
		strings.Contains(userAgentLower, "android") ||
		strings.Contains(userAgentLower, "iphone") ||
		strings.Contains(userAgentLower, "ipod") ||
		strings.Contains(userAgentLower, "blackberry") ||
		strings.Contains(userAgentLower, "windows phone") {
		return activitypb.DeviceType_DEVICE_TYPE_MOBILE
	}

	// Check for tablets
	if strings.Contains(userAgentLower, "tablet") ||
		strings.Contains(userAgentLower, "ipad") ||
		strings.Contains(userAgentLower, "kindle") ||
		strings.Contains(userAgentLower, "playbook") {
		return activitypb.DeviceType_DEVICE_TYPE_TABLET
	}

	// Default to desktop
	return activitypb.DeviceType_DEVICE_TYPE_DESKTOP
}

// GetBrowserFromUserAgent extracts browser information from user agent
func GetBrowserFromUserAgent(userAgent string) string {
	if userAgent == "" {
		return "unknown"
	}

	userAgentLower := strings.ToLower(userAgent)

	// Check for specific browsers
	if strings.Contains(userAgentLower, "chrome") && !strings.Contains(userAgentLower, "edg") {
		return "chrome"
	}
	if strings.Contains(userAgentLower, "firefox") {
		return "firefox"
	}
	if strings.Contains(userAgentLower, "safari") && !strings.Contains(userAgentLower, "chrome") {
		return "safari"
	}
	if strings.Contains(userAgentLower, "edg") {
		return "edge"
	}
	if strings.Contains(userAgentLower, "opera") || strings.Contains(userAgentLower, "opr") {
		return "opera"
	}
	if strings.Contains(userAgentLower, "internet explorer") || strings.Contains(userAgentLower, "msie") {
		return "internet_explorer"
	}

	return "unknown"
}

// GetOperatingSystemFromUserAgent extracts OS information from user agent
func GetOperatingSystemFromUserAgent(userAgent string) string {
	if userAgent == "" {
		return "unknown"
	}

	userAgentLower := strings.ToLower(userAgent)

	// Check for specific operating systems
	if strings.Contains(userAgentLower, "windows") {
		return "windows"
	}
	if strings.Contains(userAgentLower, "mac os") || strings.Contains(userAgentLower, "macos") {
		return "macos"
	}
	if strings.Contains(userAgentLower, "linux") {
		return "linux"
	}
	if strings.Contains(userAgentLower, "android") {
		return "android"
	}
	if strings.Contains(userAgentLower, "ios") || strings.Contains(userAgentLower, "iphone") || strings.Contains(userAgentLower, "ipad") {
		return "ios"
	}
	if strings.Contains(userAgentLower, "ubuntu") {
		return "ubuntu"
	}
	if strings.Contains(userAgentLower, "centos") {
		return "centos"
	}
	if strings.Contains(userAgentLower, "fedora") {
		return "fedora"
	}

	return "unknown"
}
