//go:build linux

package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/godbus/dbus/v5"
)

const (
	portalBusName          = "org.freedesktop.portal.Desktop"
	portalObjectPath       = dbus.ObjectPath("/org/freedesktop/portal/desktop")
	portalRequestInterface = "org.freedesktop.portal.Request"
	portalResponseMember   = "Response"
	portalSuccessResponse  = uint32(0)
	portalCancelResponse   = uint32(1)
)

func pickDirectory(current, title string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), folderPickerTimeout)
	defer cancel()

	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return "", fmt.Errorf("connect session bus: %w", err)
	}
	defer conn.Close()

	senderNames := conn.Names()
	if len(senderNames) == 0 {
		return "", fmt.Errorf("resolve session bus name: empty connection names")
	}

	sender := portalSenderName(senderNames)
	if sender == "" {
		return "", fmt.Errorf("resolve session bus name: no unique connection name")
	}

	token, err := portalHandleToken()
	if err != nil {
		return "", err
	}
	requestPath := portalRequestPath(sender, token)

	subscribe := func(p dbus.ObjectPath) error {
		return conn.AddMatchSignal(
			dbus.WithMatchInterface(portalRequestInterface),
			dbus.WithMatchMember(portalResponseMember),
			dbus.WithMatchObjectPath(p),
			dbus.WithMatchSender(portalBusName),
		)
	}
	if err := subscribe(requestPath); err != nil {
		return "", fmt.Errorf("subscribe portal response: %w", err)
	}
	defer func() { _ = conn.RemoveMatchSignal(
		dbus.WithMatchInterface(portalRequestInterface),
		dbus.WithMatchMember(portalResponseMember),
		dbus.WithMatchObjectPath(requestPath),
		dbus.WithMatchSender(portalBusName),
	) }()

	signals := make(chan *dbus.Signal, 8)
	conn.Signal(signals)
	defer conn.RemoveSignal(signals)

	options := map[string]dbus.Variant{
		"handle_token": dbus.MakeVariant(token),
		"directory":    dbus.MakeVariant(true),
		"modal":        dbus.MakeVariant(true),
	}
	if current != "" {
		options["current_folder"] = dbus.MakeVariant(currentFolderURI(current))
	}

	var handle dbus.ObjectPath
	call := conn.Object(portalBusName, portalObjectPath).CallWithContext(
		ctx,
		"org.freedesktop.portal.FileChooser.OpenFile",
		0,
		"",
		title,
		options,
	)
	if call.Err != nil {
		return "", fmt.Errorf("call portal file chooser: %w", call.Err)
	}
	if err := call.Store(&handle); err != nil {
		return "", fmt.Errorf("decode portal file chooser response: %w", err)
	}

	subscribed := requestPath
	if handle != requestPath {
		if err := subscribe(handle); err != nil {
			return "", fmt.Errorf("subscribe actual portal response: %w", err)
		}
		subscribed = handle
		defer func() { _ = conn.RemoveMatchSignal(
			dbus.WithMatchInterface(portalRequestInterface),
			dbus.WithMatchMember(portalResponseMember),
			dbus.WithMatchObjectPath(subscribed),
			dbus.WithMatchSender(portalBusName),
		) }()
	}

	for {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("wait for portal response: %w", ctx.Err())
		case sig := <-signals:
			if sig == nil || sig.Path != handle || sig.Name != portalRequestInterface+"."+portalResponseMember {
				continue
			}
			return parsePortalFolderSignal(sig)
		}
	}
}

func portalSenderName(names []string) string {
	for _, name := range names {
		if strings.HasPrefix(name, ":") {
			return name
		}
	}
	return ""
}

func currentFolderURI(path string) string {
	return (&url.URL{Scheme: "file", Path: filepath.ToSlash(path)}).String()
}

func parsePortalFolderSignal(sig *dbus.Signal) (string, error) {
	var (
		response uint32
		results  map[string]dbus.Variant
	)
	if err := dbus.Store(sig.Body, &response, &results); err != nil {
		return "", fmt.Errorf("decode portal signal: %w", err)
	}

	switch response {
	case portalSuccessResponse:
	case portalCancelResponse:
		return "", ErrFolderPickerCancelled
	default:
		return "", fmt.Errorf("portal returned response %d", response)
	}

	urisVar, ok := results["uris"]
	if !ok {
		return "", fmt.Errorf("portal returned no folder URI")
	}

	var uris []string
	if err := urisVar.Store(&uris); err != nil {
		return "", fmt.Errorf("decode portal folder URI: %w", err)
	}
	if len(uris) == 0 {
		return "", fmt.Errorf("portal returned an empty folder URI list")
	}

	path, err := fileURIPath(uris[0])
	if err != nil {
		return "", err
	}
	return cleanAbsPath(path), nil
}

func portalHandleToken() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate portal token: %w", err)
	}
	return "volvid" + hex.EncodeToString(buf), nil
}

func portalRequestPath(sender, token string) dbus.ObjectPath {
	replacer := strings.NewReplacer(":", "", ".", "_", "-", "_")
	return dbus.ObjectPath("/org/freedesktop/portal/desktop/request/" + replacer.Replace(sender) + "/" + token)
}

func fileURIPath(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", fmt.Errorf("parse folder URI: %w", err)
	}
	if !strings.EqualFold(u.Scheme, "file") {
		return "", fmt.Errorf("unsupported folder URI scheme: %s", u.Scheme)
	}
	if strings.TrimSpace(u.Host) != "" && !strings.EqualFold(u.Host, "localhost") {
		return "", fmt.Errorf("unsupported folder URI host: %s", u.Host)
	}
	path, err := url.PathUnescape(u.Path)
	if err != nil {
		return "", fmt.Errorf("decode folder URI path: %w", err)
	}
	return path, nil
}
