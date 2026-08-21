package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"
)

type machineFingerprintRecord struct {
	OS           string `json:"os"`
	Architecture string `json:"architecture"`
	Hostname     string `json:"hostname,omitempty"`
	Chip         string `json:"chip,omitempty"`
	MemoryBytes  int64  `json:"memory_bytes,omitempty"`
}

type localctlFingerprintRecord struct {
	Version  string `json:"version"`
	Revision string `json:"revision,omitempty"`
	Dirty    bool   `json:"dirty,omitempty"`
}

type runtimeFingerprintRecord struct {
	Kind          string `json:"kind"`
	URL           string `json:"url"`
	PID           int    `json:"pid,omitempty"`
	Executable    string `json:"executable,omitempty"`
	ExecutableKey string `json:"executable_metadata_key,omitempty"`
	Version       string `json:"version,omitempty"`
	StartedAt     string `json:"started_at,omitempty"`
}

type artifactIdentityRecord struct {
	SHA256       string `json:"sha256,omitempty"`
	Quantization string `json:"quantization,omitempty"`
}

type artifactDigestCache struct {
	Path       string    `json:"path"`
	Size       int64     `json:"size_bytes"`
	ModifiedAt time.Time `json:"modified_at"`
	SHA256     string    `json:"sha256"`
}

var quantPattern = regexp.MustCompile(`(?i)(IQ[0-9]+_[A-Z0-9_]+|Q[0-9]+(?:_[A-Z0-9]+)+|Q[0-9]+_[0-9]+)`)
var machineOnce sync.Once
var machineCached machineFingerprintRecord
var runtimeVersionMu sync.Mutex
var runtimeVersions = map[string]string{}
var artifactDigestMu sync.Mutex
var artifactDigests = map[string]string{}

func machineFingerprint() machineFingerprintRecord {
	machineOnce.Do(func() {
		record := machineFingerprintRecord{OS: runtime.GOOS, Architecture: runtime.GOARCH}
		record.Hostname, _ = os.Hostname()
		if runtime.GOOS == "darwin" {
			record.Chip = commandOutput("/usr/sbin/sysctl", "-n", "machdep.cpu.brand_string")
			if record.Chip == "" {
				record.Chip = commandOutput("/usr/sbin/sysctl", "-n", "hw.model")
			}
			if value := commandOutput("/usr/sbin/sysctl", "-n", "hw.memsize"); value != "" {
				record.MemoryBytes, _ = strconv.ParseInt(strings.TrimSpace(value), 10, 64)
			}
		} else if runtime.GOOS == "linux" {
			if file, err := os.Open("/proc/meminfo"); err == nil {
				scanner := bufio.NewScanner(file)
				for scanner.Scan() {
					fields := strings.Fields(scanner.Text())
					if len(fields) >= 2 && fields[0] == "MemTotal:" {
						kb, _ := strconv.ParseInt(fields[1], 10, 64)
						record.MemoryBytes = kb * 1024
						break
					}
				}
				_ = file.Close()
			}
			record.Chip = commandOutput("uname", "-m")
		}
		machineCached = record
	})
	return machineCached
}

func localctlFingerprint() localctlFingerprintRecord {
	record := localctlFingerprintRecord{Version: localctlVersion}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return record
	}
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			record.Revision = setting.Value
		case "vcs.modified":
			record.Dirty = setting.Value == "true"
		}
	}
	return record
}

func runtimeFingerprint(model modelArtifact) runtimeFingerprintRecord {
	record := runtimeFingerprintRecord{Kind: "llama.cpp", URL: runtimeURL}
	state, err := readRuntimeState()
	if err != nil || state.Model != model.Path {
		return record
	}
	record.PID = state.PID
	record.Executable = state.Executable
	record.StartedAt = state.StartedAt.Format(time.RFC3339Nano)
	if info, statErr := os.Stat(state.Executable); statErr == nil {
		record.ExecutableKey = textSHA256(fmt.Sprintf("%s\n%d\n%d", state.Executable, info.Size(), info.ModTime().UnixNano()))
	}
	runtimeVersionMu.Lock()
	version, ok := runtimeVersions[record.ExecutableKey]
	runtimeVersionMu.Unlock()
	if !ok {
		version = firstLine(commandOutput(state.Executable, "--version"))
		runtimeVersionMu.Lock()
		runtimeVersions[record.ExecutableKey] = version
		runtimeVersionMu.Unlock()
	}
	record.Version = version
	return record
}

func commandOutput(name string, args ...string) string {
	output, err := exec.Command(name, args...).CombinedOutput()
	if err != nil && len(output) == 0 {
		return ""
	}
	return strings.TrimSpace(string(output))
}

func firstLine(value string) string {
	if index := strings.IndexByte(value, '\n'); index >= 0 {
		return strings.TrimSpace(value[:index])
	}
	return strings.TrimSpace(value)
}

func quantizationFromName(name string) string {
	return quantPattern.FindString(strings.ToUpper(name))
}

func artifactIdentity(model modelArtifact) artifactIdentityRecord {
	digest, _ := cachedArtifactSHA256(model)
	return artifactIdentityRecord{SHA256: digest, Quantization: quantizationFromName(model.Name)}
}

func artifactCacheRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".localctl", "artifacts"), nil
}

func cachedArtifactSHA256(model modelArtifact) (string, error) {
	info, err := os.Stat(model.Path)
	if err != nil {
		return "", err
	}
	key := textSHA256(fmt.Sprintf("%s\n%d\n%d", model.Path, info.Size(), info.ModTime().UnixNano()))
	artifactDigestMu.Lock()
	if digest, ok := artifactDigests[key]; ok {
		artifactDigestMu.Unlock()
		return digest, nil
	}
	artifactDigestMu.Unlock()
	root, err := artifactCacheRoot()
	if err != nil {
		return "", err
	}
	path := filepath.Join(root, key+".json")
	if data, readErr := os.ReadFile(path); readErr == nil {
		var cached artifactDigestCache
		if json.Unmarshal(data, &cached) == nil && cached.Path == model.Path && cached.Size == info.Size() && cached.ModifiedAt.Equal(info.ModTime()) {
			artifactDigestMu.Lock()
			artifactDigests[key] = cached.SHA256
			artifactDigestMu.Unlock()
			return cached.SHA256, nil
		}
	}
	file, err := os.Open(model.Path)
	if err != nil {
		return "", err
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		_ = file.Close()
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	digest := hex.EncodeToString(hash.Sum(nil))
	artifactDigestMu.Lock()
	artifactDigests[key] = digest
	artifactDigestMu.Unlock()
	cache := artifactDigestCache{Path: model.Path, Size: info.Size(), ModifiedAt: info.ModTime(), SHA256: digest}
	if err := os.MkdirAll(root, 0700); err == nil {
		if data, marshalErr := json.MarshalIndent(cache, "", "  "); marshalErr == nil {
			_ = os.WriteFile(path, data, 0600)
		}
	}
	return digest, nil
}
