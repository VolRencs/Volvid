package bot

import (
	"fmt"

	app "YouTubeBuild/internal/app"
)

func (b *Bot) startConfiguredDownload(chatID int64, sess *Session) error {
	req, err := b.prepareConfiguredDownload(chatID, sess)
	if err != nil {
		return err
	}

	b.beginConfiguredDownload(chatID, sess, req)
	go b.runDownload(chatID, sess, req)
	return nil
}

func (b *Bot) prepareConfiguredDownload(chatID int64, sess *Session) (app.DownloadRequest, error) {
	req, err := b.buildDownloadRequest(sess)
	if err != nil {
		return app.DownloadRequest{}, err
	}

	if err := b.checkDownloadSizeLimit(chatID, sess, req); err != nil {
		return app.DownloadRequest{}, err
	}
	return req, nil
}

func (b *Bot) checkDownloadSizeLimit(chatID int64, sess *Session, req app.DownloadRequest) error {
	snap := sess.snapshot()
	estimate, err := app.EstimateDownloadSize(req)
	if err != nil {
		b.logError(fmt.Sprintf("download size estimate %s", logChatUser(chatID, snap.UserID)), err)
		return nil
	}
	limit := b.downloadSizeLimit(snap.UserID)
	if estimate.TotalBytes <= limit {
		return nil
	}

	b.logf("download limit exceeded before start %s estimated=%d limit=%d", logChatUser(chatID, snap.UserID), estimate.TotalBytes, limit)
	b.notifyDownloadSizeLimitExceeded(chatID, snap.StatusMsgID, snap.UserID)
	return errDownloadLimitExceeded
}

func (b *Bot) beginConfiguredDownload(chatID int64, sess *Session, req app.DownloadRequest) {
	sess.mutate(func(s *Session) {
		s.State = StateDownloading
	})

	snap := sess.snapshot()
	b.logf(
		"download start %s mode=%d single=%t playlist=%t entries=%d workdir=%q",
		logChatUser(chatID, snap.UserID),
		req.Profile.Mode,
		req.ForceSingle,
		req.PlaylistInfo != nil && !req.ForceSingle,
		len(req.Entries),
		snap.WorkDir,
	)
	b.removeKb(chatID, snap.StatusMsgID)
	b.edit(chatID, snap.StatusMsgID, b.downloadStartText(sess))
}
