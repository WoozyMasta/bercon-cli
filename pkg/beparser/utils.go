package beparser

import (
	"net"
	"strconv"
	"strings"
)

func parseAddress(addr string) (string, uint16) {
	address := strings.Split(addr, ":")
	ip := strings.TrimSpace(address[0])

	if !isValidIPv4(ip) {
		ip = "invalid"
	}

	var port int
	if len(address) != 2 {
		return ip, 0
	}

	port, err := strconv.Atoi(address[1])
	if err != nil {
		return ip, 0
	}

	return ip, uint16(port)
}

func isValidIPv4(ip string) bool {
	ipAddr := net.ParseIP(ip)
	return ipAddr != nil && ipAddr.To4() != nil
}

func getMinutes(line string) int {
	if line == "perm" {
		return -1
	}

	if line == "-" {
		return 0
	}

	minutes, err := strconv.Atoi(line)
	if err != nil {
		return 0
	}

	return minutes
}
