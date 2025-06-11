package utils

import (
	"strings"
	"crypto/rand"
	"math/big"
	"net"
	"time"
	"fmt"
)

// zero out data from memory 
func ZeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

// generate randomized string of length 
func Generate(length int) (string, error) {
	srcString := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdef0123456789"
	srcLen := int64(len(srcString))

	var result strings.Builder
	for i := 0; i < length; i++ {
		
		// Generate a random number within the length of srcString
		index, err := rand.Int(rand.Reader, big.NewInt(srcLen))
		if err != nil {
			return "", err
		}
		// Use index as the position in srcString
		result.WriteByte(srcString[index.Int64()])
	}
	return result.String(), nil
}

//getLocalIP tries to get a non-loopback IP address

func GetLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "unknown"
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return "unknown"
}




// TCPDistanceEstimate estimates the approximate distance (in metres)
// based on a round-trip time in nanoseconds.
func DistanceEstimate(rttNano uint64) float64 {
    const speedOfLight = 3e8 // metres per second
    rttSeconds := float64(rttNano) / 1e9
    return (speedOfLight * rttSeconds) / 2
}

// FormatDistance prints the result nicely in metres or kilometres
func FormatDistance(metres float64) string {
    if metres >= 1000 {
        return fmt.Sprintf("%.2f kilometres", metres/1000)
    }
    return fmt.Sprintf("%.2f metres", metres)
}
