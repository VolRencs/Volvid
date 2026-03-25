package app

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type UpdateInfo struct {
	Latest string
	DlURL  string
}

func assetName() string {
	switch {
	case IsWindows:
		return "VolRenDownloader.exe"
	case Arch == "arm64":
		return "VolRenDownloader_linux_arm64"
	default:
		return "VolRenDownloader_linux_amd64"
	}
}

func CheckUpdate() *UpdateInfo {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	return CheckUpdateContext(ctx)
}

func CheckUpdateContext(ctx context.Context) *UpdateInfo {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubAPIURL, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", "VolRenDownloader/"+Version)
	resp, err := doSafeRequest(ctx, apiClient, req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	var data map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil
	}
	latest := strings.TrimPrefix(strVal(data, "tag_name", ""), "v")
	if latest == "" || !versionGT(latest, Version) {
		return nil
	}
	assets, _ := data["assets"].([]any)
	want := assetName()
	for _, a := range assets {
		asset, ok := a.(map[string]any)
		if !ok {
			continue
		}
		if strVal(asset, "name", "") == want {
			return &UpdateInfo{
				Latest: latest,
				DlURL:  strVal(asset, "browser_download_url", ""),
			}
		}
	}
	return nil
}

func ApplyUpdate(info *UpdateInfo, ch chan<- FileProgress) error {
	return ApplyUpdateFor(LoadLocale(), info, ch)
}

func ApplyUpdateFor(l Locale, info *UpdateInfo, ch chan<- FileProgress) error {
    exe, err := os.Executable()
    if err != nil {
        return fmt.Errorf("путь к исполняемому файлу: %w", err)
    }
    dest, err := filepath.Abs(exe)
    if err != nil {
        return fmt.Errorf("абсолютный путь: %w", err)
    }
    if IsWindows {
        tmp := strings.TrimSuffix(dest, ".exe") + ".new.exe"
        if err := DownloadFile(info.DlURL, tmp, l, ch); err != nil {
            return err
        }
        if err := applyUpdatePlatform(tmp, dest); err != nil {
            return err
        }
        os.Exit(0) 
    }
    tmp := dest + ".new"
    if err := DownloadFile(info.DlURL, tmp, l, ch); err != nil {
        return err
    }
    return applyUpdatePlatform(tmp, dest)
}

func versionGT(a, b string) bool {
	parse := func(s string) [3]int {
		var v [3]int
		for i, p := range strings.SplitN(s, ".", 3) {
			v[i], _ = strconv.Atoi(p)
		}
		return v
	}
	av, bv := parse(a), parse(b)
	for i := range 3 {
		if c := cmp.Compare(av[i], bv[i]); c != 0 {
			return c > 0
		}
	}
	return false
}
