// Package selfupdate installs verified binary releases without a source checkout.
package selfupdate

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var versionPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-(alpha|beta|rc)\.(0|[1-9][0-9]*))?$`)

// Version normalizes and validates supported release versions.
func Version(input string) (string, error) {
	v := strings.TrimPrefix(input, "v")
	if !versionPattern.MatchString(v) {
		return "", fmt.Errorf("invalid release version: %s", input)
	}
	return v, nil
}

// Updater downloads release assets through an injectable HTTP client.
type Updater struct {
	Client                                      *http.Client
	APIBase, DownloadBase, Repo, Platform, Arch string
}

func (u Updater) defaults() Updater {
	if u.Client == nil {
		u.Client = &http.Client{Timeout: 3 * time.Minute}
	}
	if u.APIBase == "" {
		u.APIBase = "https://api.github.com"
	}
	if u.DownloadBase == "" {
		u.DownloadBase = "https://github.com"
	}
	if u.Repo == "" {
		u.Repo = "LBYPatrick/ashley"
	}
	return u
}
func (u Updater) get(ctx context.Context, url string, limit int64) ([]byte, error) {
	u = u.defaults()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Ashley binary updater")
	response, err := u.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: %s", url, response.Status)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("download exceeds size limit")
	}
	return data, nil
}

// Latest resolves the most recent stable GitHub release.
func (u Updater) Latest(ctx context.Context) (string, error) {
	u = u.defaults()
	data, err := u.get(ctx, u.APIBase+"/repos/"+u.Repo+"/releases/latest", 1<<20)
	if err != nil {
		return "", err
	}
	var release struct {
		Tag string `json:"tag_name"`
	}
	if err := json.Unmarshal(data, &release); err != nil {
		return "", err
	}
	return Version(release.Tag)
}

// Fetch validates a release checksum and extracts only its regular ash executable.
func (u Updater) Fetch(ctx context.Context, version string) ([]byte, error) {
	u = u.defaults()
	version, err := Version(version)
	if err != nil {
		return nil, err
	}
	if (u.Platform != "darwin" && u.Platform != "linux") || (u.Arch != "amd64" && u.Arch != "arm64") {
		return nil, fmt.Errorf("unsupported release platform: %s/%s", u.Platform, u.Arch)
	}
	name := fmt.Sprintf("ashley-%s-%s-%s.tar.gz", version, u.Platform, u.Arch)
	base := u.DownloadBase + "/" + u.Repo + "/releases/download/v" + version + "/" + name
	manifest, err := u.get(ctx, base+".sha256", 4096)
	if err != nil {
		return nil, err
	}
	fields := strings.Fields(string(manifest))
	if len(fields) != 2 || fields[1] != name {
		return nil, fmt.Errorf("invalid checksum manifest")
	}
	expected, err := hex.DecodeString(fields[0])
	if err != nil || len(expected) != sha256.Size {
		return nil, fmt.Errorf("invalid SHA-256 checksum")
	}
	archive, err := u.get(ctx, base, 128<<20)
	if err != nil {
		return nil, err
	}
	actual := sha256.Sum256(archive)
	if !bytes.Equal(expected, actual[:]) {
		return nil, fmt.Errorf("release checksum mismatch")
	}
	reader, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	// Bound the total expanded archive, including unexpected members.
	tarReader := tar.NewReader(io.LimitReader(reader, 256<<20))
	var executable []byte
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if header.Name != "ash" && header.Name != "LICENSE" {
			return nil, fmt.Errorf("unexpected release member: %s", header.Name)
		}
		if header.Typeflag != tar.TypeReg || header.Size < 0 || header.Size > 128<<20 {
			return nil, fmt.Errorf("invalid release member: %s", header.Name)
		}
		if header.Name == "ash" {
			if executable != nil {
				return nil, fmt.Errorf("duplicate executable in archive")
			}
			executable, err = io.ReadAll(tarReader)
			if err != nil {
				return nil, err
			}
		}
	}
	if len(executable) == 0 {
		return nil, fmt.Errorf("release archive has no executable")
	}
	return executable, nil
}

// Install validates the candidate version and atomically replaces the destination.
// A destination symlink is replaced itself; its old source checkout is untouched.
func Install(ctx context.Context, destination, version string, data []byte) error {
	version, err := Version(version)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
		return err
	}
	candidate, err := os.CreateTemp(filepath.Dir(destination), ".ash-update-*")
	if err != nil {
		return err
	}
	defer os.Remove(candidate.Name())
	_, writeErr := candidate.Write(data)
	syncErr := candidate.Sync()
	closeErr := candidate.Close()
	if err := errors.Join(writeErr, syncErr, closeErr); err != nil {
		return err
	}
	if err := os.Chmod(candidate.Name(), 0755); err != nil {
		return err
	}
	verifyCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	output, err := exec.CommandContext(verifyCtx, candidate.Name(), "--version").CombinedOutput()
	if err != nil {
		return fmt.Errorf("candidate validation: %w", err)
	}
	if strings.TrimSpace(string(output)) != "ashley "+version {
		return fmt.Errorf("downloaded binary version mismatch: %s", strings.TrimSpace(string(output)))
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return os.Rename(candidate.Name(), destination)
}
