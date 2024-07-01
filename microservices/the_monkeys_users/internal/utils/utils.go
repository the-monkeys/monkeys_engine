package utils

import "github.com/the-monkeys/the_monkeys/constants"

func IpClientConvert(ip, client string) (string, string) {
	if ip == "" {
		ip = "127.0.0.1"
	}

	for _, cli := range constants.Clients {
		if client == cli {
			return ip, client
		}
	}

	client = "Others"

	return ip, client
}
