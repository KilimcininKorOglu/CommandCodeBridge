package fingerprint

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
)

// CPUEntry represents a CPU model with core count
type CPUEntry struct {
	Model string
	Cores int
}

// Fingerprint data for anti-detection
var (
	FINGERPRINT_CPUS = []CPUEntry{
		{Model: "12th Gen Intel(R) Core(TM) i7-12650H", Cores: 10},
		{Model: "12th Gen Intel(R) Core(TM) i5-12400F", Cores: 6},
		{Model: "12th Gen Intel(R) Core(TM) i9-12900K", Cores: 16},
		{Model: "13th Gen Intel(R) Core(TM) i7-13700K", Cores: 16},
		{Model: "13th Gen Intel(R) Core(TM) i5-13600K", Cores: 14},
		{Model: "13th Gen Intel(R) Core(TM) i9-13900K", Cores: 24},
		{Model: "Intel(R) Core(TM) Ultra 7 155H", Cores: 16},
		{Model: "Intel(R) Core(TM) Ultra 9 285H", Cores: 16},
		{Model: "Intel(R) Core(TM) i9-14900K", Cores: 24},
		{Model: "Intel(R) Core(TM) i7-14700K", Cores: 20},
		{Model: "Intel(R) Core(TM) i5-14400F", Cores: 10},
		{Model: "Intel(R) Core(TM) i5-14500", Cores: 14},
		{Model: "Intel(R) Core(TM) i5-13400F", Cores: 10},
		{Model: "Intel(R) Core(TM) i7-14700", Cores: 20},
		{Model: "Intel(R) Core(TM) i7-12700K", Cores: 12},
		{Model: "Intel(R) Core(TM) i5-12600K", Cores: 10},
		{Model: "Intel(R) Core(TM) i9-12900KF", Cores: 16},
		{Model: "Intel(R) Core(TM) i7-12700KF", Cores: 12},
		{Model: "Intel(R) Core(TM) i5-12600KF", Cores: 10},
		{Model: "AMD Ryzen 7 7800X3D", Cores: 8},
		{Model: "AMD Ryzen 9 7950X", Cores: 16},
		{Model: "AMD Ryzen 5 7600", Cores: 6},
		{Model: "AMD Ryzen 9 7900X", Cores: 12},
		{Model: "AMD Ryzen 7 5800X3D", Cores: 8},
		{Model: "AMD Ryzen 9 7950X3D", Cores: 16},
		{Model: "AMD Ryzen 7 7700X", Cores: 8},
		{Model: "AMD Ryzen 5 7600X", Cores: 6},
		{Model: "AMD Ryzen 9 7900X3D", Cores: 12},
		{Model: "AMD Ryzen 7 5700X", Cores: 8},
		{Model: "AMD Ryzen 5 5600X", Cores: 6},
		{Model: "AMD Ryzen 9 5900X", Cores: 12},
		{Model: "AMD Ryzen 7 5800X", Cores: 8},
		{Model: "AMD Ryzen 5 5600", Cores: 6},
		{Model: "AMD Ryzen 9 5950X", Cores: 16},
		{Model: "AMD Ryzen 7 5700G", Cores: 8},
		{Model: "AMD Ryzen 5 5600G", Cores: 6},
	}
	FINGERPRINT_MEMS = []int{8, 12, 16, 24, 32, 48, 64, 96, 128}
	FINGERPRINT_TZS = []string{
		"America/New_York", "America/Chicago", "America/Los_Angeles", "America/Toronto",
		"America/Denver", "America/Phoenix", "America/Mexico_City", "America/Bogota",
		"America/Sao_Paulo", "America/Buenos_Aires", "America/Lima", "America/Santiago",
		"Europe/London", "Europe/Berlin", "Europe/Paris", "Europe/Moscow",
		"Europe/Madrid", "Europe/Rome", "Europe/Amsterdam", "Europe/Brussels",
		"Europe/Vienna", "Europe/Zurich", "Europe/Stockholm", "Europe/Oslo",
		"Europe/Copenhagen", "Europe/Warsaw", "Europe/Prague", "Europe/Budapest",
	}
	FINGERPRINT_MAC_COUNT_RANGE = []int{2, 3, 4, 5}
)

// Fingerprint represents machine fingerprint
type Fingerprint struct {
	Thumbmark  string
	Components FingerprintComponents
}

// FingerprintComponents represents fingerprint components
type FingerprintComponents struct {
	MachineIdHash    string
	MacHashes        []string
	OsUserHash       string
	HostnameHash     string
	GitEmailHash     string
	Platform         string
	Arch             string
	OsRelease        string
	CpuModel         string
	CpuCount         int
	MemGiB           int
	IsContainer      bool
	Timezone         string
	Runtime          string
	CollectorVersion int
}

// Generate generates a fake machine fingerprint for anti-detection
func Generate() (*Fingerprint, error) {
	cpuIdx, _ := randInt(len(FINGERPRINT_CPUS))
	cpuEntry := FINGERPRINT_CPUS[cpuIdx]

	memIdx, _ := randInt(len(FINGERPRINT_MEMS))
	memGiB := FINGERPRINT_MEMS[memIdx]

	tzIdx, _ := randInt(len(FINGERPRINT_TZS))
	tz := FINGERPRINT_TZS[tzIdx]

	macCountIdx, _ := randInt(len(FINGERPRINT_MAC_COUNT_RANGE))
	macCount := FINGERPRINT_MAC_COUNT_RANGE[macCountIdx]

	macHashes := make([]string, macCount)
	for i := 0; i < macCount; i++ {
		hash, _ := SHA256(RandomHex(32))
		macHashes[i] = hash
	}

	machineIdHash, _ := SHA256(RandomHex(32))
	osUserHash, _ := SHA256(RandomHex(16))
	hostnameHash, _ := SHA256(RandomHex(16))
	gitEmailHash, _ := SHA256(RandomHex(16))

	thumbData := fmt.Sprintf("%s|%s|%s|%s|%s|win32|10.0.22631|%s|%d|%d", machineIdHash, macHashes[0], osUserHash, hostnameHash, gitEmailHash, cpuEntry.Model, cpuEntry.Cores, memGiB)
	thumbmark, _ := SHA256(thumbData)

	return &Fingerprint{
		Thumbmark: thumbmark,
		Components: FingerprintComponents{
			MachineIdHash:    machineIdHash,
			MacHashes:        macHashes,
			OsUserHash:       osUserHash,
			HostnameHash:     hostnameHash,
			GitEmailHash:     gitEmailHash,
			Platform:         "win32",
			Arch:             "x64",
			OsRelease:        "10.0.22631",
			CpuModel:         cpuEntry.Model,
			CpuCount:         cpuEntry.Cores,
			MemGiB:           memGiB,
			IsContainer:      false,
			Timezone:         tz,
			Runtime:          "cli",
			CollectorVersion: 1,
		},
	}, nil
}

// randInt returns a random non-negative integer less than max
func randInt(max int) (int, error) {
	if max <= 0 {
		return 0, nil
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return 0, err
	}
	return int(n.Int64()), nil
}

// SHA256 computes SHA256 hash of a string
func SHA256(data string) (string, error) {
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:]), nil
}

// RandomHex generates random hex string of specified length
func RandomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}
