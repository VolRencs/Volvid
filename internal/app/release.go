package app

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

type UpdateInfo struct {
	Latest string
	DlURL  string
}

func assetName() (string, error) {
	platform, err := currentPlatform()
	if err != nil {
		return "", err
	}
	if platform.UpdateAsset == "" {
		return "", fmt.Errorf("release asset name is empty")
	}
	return platform.UpdateAsset, nil
}

func CheckUpdate(env *Env) *UpdateInfo {
	ctx, cancel := context.WithTimeout(context.Background(), apiClientTimeout)
	defer cancel()
	return CheckUpdateContext(env, ctx)
}

func CheckUpdateContext(env *Env, ctx context.Context) *UpdateInfo {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubAPIURL, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", "Volvid/"+Version)
	resp, err := doSafeRequest(ctx, env.apiClient, req)
	if err != nil {
		fmt.Printf("update check: request failed: %v\n", err)
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	var data map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		fmt.Printf("update check: decode failed: %v\n", err)
		return nil
	}
	latest := strings.TrimPrefix(mapString(data, "tag_name", ""), "v")
	if latest == "" || !versionGT(latest, Version) {
		return nil
	}
	assets, _ := data["assets"].([]any)
	want, err := assetName()
	if err != nil {
		return nil
	}
	for _, a := range assets {
		asset, ok := a.(map[string]any)
		if !ok {
			continue
		}
		if mapString(asset, "name", "") == want {
			dlURL := mapString(asset, "browser_download_url", "")
			if !validUpdateDownloadURL(dlURL, want) {
				continue
			}
			return &UpdateInfo{
				Latest: latest,
				DlURL:  dlURL,
			}
		}
	}
	return nil
}

func validUpdateDownloadURL(raw, assetName string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u == nil {
		return false
	}
	if u.User != nil || !strings.EqualFold(u.Scheme, "https") || !strings.EqualFold(u.Hostname(), "github.com") {
		return false
	}

	cleanPath := path.Clean("/" + strings.TrimPrefix(u.EscapedPath(), "/"))
	if path.Base(cleanPath) != assetName {
		return false
	}
	return strings.HasPrefix(cleanPath, "/VolRencs/Volvid/releases/download/")
}

func ApplyUpdateFor(env *Env, ctx context.Context, l Locale, info *UpdateInfo, ch chan<- FileProgress) error {
	if info == nil || strings.TrimSpace(info.DlURL) == "" {
		return fmt.Errorf("update info is empty")
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("путь к исполняемому файлу: %w", err)
	}
	dest, err := filepath.Abs(exe)
	if err != nil {
		return fmt.Errorf("абсолютный путь: %w", err)
	}
	if env.IsWindows {
		tmp := strings.TrimSuffix(dest, ".exe") + ".new.exe"
		if err := DownloadFileContext(env, ctx, info.DlURL, tmp, l, ch); err != nil {
			return err
		}
		return applyUpdatePlatform(tmp, dest)
	}
	tmp := dest + ".new"
	if err := DownloadFileContext(env, ctx, info.DlURL, tmp, l, ch); err != nil {
		return err
	}
	return applyUpdatePlatform(tmp, dest)
}

func versionGT(a, b string) bool {
	parse := func(s string) [4]int {
		var v [4]int
		for i, p := range strings.SplitN(s, ".", 4) {
			v[i], _ = strconv.Atoi(p)
		}
		return v
	}
	av, bv := parse(a), parse(b)
	for i := range 4 {
		if c := cmp.Compare(av[i], bv[i]); c != 0 {
			return c > 0
		}
	}
	return false
}
