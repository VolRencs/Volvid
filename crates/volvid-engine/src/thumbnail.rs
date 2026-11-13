use std::sync::Arc;

use anyhow::{Context, Result, bail};
use image::{ImageReader, Limits};
use tokio_util::sync::CancellationToken;

const MAX_THUMBNAIL_BYTES: usize = 4 << 20;
const MAX_IMAGE_DIMENSION: u32 = 4096;
const MAX_DECODE_ALLOC: u64 = 64 << 20;

/// Decoded RGBA preview shared between the engine and the GUI.
#[derive(Clone, Debug)]
pub struct PreviewImage {
    pub width: u32,
    pub height: u32,
    pub rgba: Arc<[u8]>,
}

/// Small 16:9 YouTube thumbnail that can be fetched before ytrust finishes.
pub fn youtube_preview_url(video_id: &str) -> String {
    let id: String = url::form_urlencoded::byte_serialize(video_id.as_bytes()).collect();
    format!("https://i.ytimg.com/vi/{id}/mqdefault.jpg")
}

/// Fetch, decode and downscale a thumbnail without blocking a Tokio worker.
pub async fn fetch_preview(
    client: &reqwest::Client,
    url: &str,
    target_width: u32,
    cancel: Option<&CancellationToken>,
) -> Result<Arc<PreviewImage>> {
    let request = client
        .get(url)
        .header(
            reqwest::header::ACCEPT,
            "image/webp,image/jpeg,image/png,*/*;q=0.8",
        )
        .header(reqwest::header::REFERER, "https://www.youtube.com/");
    let response = if let Some(cancel) = cancel {
        tokio::select! {
            _ = cancel.cancelled() => bail!("operation cancelled"),
            response = request.send() => response?,
        }
    } else {
        request.send().await?
    };
    if !response.status().is_success() {
        bail!("HTTP {} for {url}", response.status());
    }
    if let Some(length) = response.content_length()
        && length > MAX_THUMBNAIL_BYTES as u64
    {
        bail!("thumbnail is too large");
    }
    if let Some(content_type) = response
        .headers()
        .get(reqwest::header::CONTENT_TYPE)
        .and_then(|value| value.to_str().ok())
        && !content_type.to_ascii_lowercase().starts_with("image/")
    {
        bail!("unexpected thumbnail content type: {content_type}");
    }

    let bytes = crate::http::read_body(
        response,
        MAX_THUMBNAIL_BYTES,
        cancel,
        "thumbnail is too large",
    )
    .await?;
    if bytes.is_empty() {
        bail!("thumbnail response is empty");
    }

    let target_width = target_width.clamp(1, MAX_IMAGE_DIMENSION);
    tokio::task::spawn_blocking(move || decode(&bytes, target_width))
        .await
        .context("thumbnail decode task failed")?
        .map(Arc::new)
}

fn decode(bytes: &[u8], target_width: u32) -> Result<PreviewImage> {
    let mut reader = ImageReader::new(std::io::Cursor::new(bytes))
        .with_guessed_format()
        .context("detect thumbnail format")?;
    let mut limits = Limits::default();
    limits.max_image_width = Some(MAX_IMAGE_DIMENSION);
    limits.max_image_height = Some(MAX_IMAGE_DIMENSION);
    limits.max_alloc = Some(MAX_DECODE_ALLOC);
    reader.limits(limits);
    let image = reader.decode().context("decode thumbnail")?;
    let image = if image.width() > target_width {
        let height =
            ((image.height() as u64 * target_width as u64) / image.width() as u64).max(1) as u32;
        image.thumbnail(target_width, height)
    } else {
        image
    };
    let rgba = image.to_rgba8();
    let (width, height) = rgba.dimensions();
    Ok(PreviewImage {
        width,
        height,
        rgba: Arc::from(rgba.into_raw()),
    })
}

#[cfg(test)]
mod tests {
    use super::{decode, youtube_preview_url};
    use image::{ImageFormat, Rgba, RgbaImage};
    use std::io::Cursor;

    #[test]
    fn builds_instant_youtube_preview_url() {
        assert_eq!(
            youtube_preview_url("dQw4w9WgXcQ"),
            "https://i.ytimg.com/vi/dQw4w9WgXcQ/mqdefault.jpg"
        );
    }

    #[test]
    fn decodes_and_downscales() {
        let image = RgbaImage::from_pixel(100, 50, Rgba([10, 20, 30, 255]));
        let mut bytes = Vec::new();
        image
            .write_to(&mut Cursor::new(&mut bytes), ImageFormat::Png)
            .unwrap();
        let preview = decode(&bytes, 40).unwrap();
        assert_eq!(preview.width, 40);
        assert_eq!(preview.height, 20);
        assert_eq!(preview.rgba.len(), 40 * 20 * 4);
    }
}
