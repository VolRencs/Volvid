use std::fs::File;
use std::io::Read;
use std::path::Path;

use anyhow::{Result, bail};
use sha2::{Digest, Sha256};

pub fn sha256_file(path: &Path) -> Result<String> {
    let mut file = File::open(path)?;
    let mut hasher = Sha256::new();
    let mut buffer = vec![0u8; 1 << 20];
    loop {
        let read = file.read(&mut buffer)?;
        if read == 0 {
            break;
        }
        hasher.update(&buffer[..read]);
    }
    Ok(hex::encode(hasher.finalize()))
}

pub fn verify_sha256(path: &Path, expected: &str) -> Result<()> {
    let expected = expected.trim().to_ascii_lowercase();
    if !is_sha256(&expected) {
        bail!("invalid SHA-256 checksum");
    }
    let actual = sha256_file(path)?;
    if actual != expected {
        bail!("sha256 mismatch for {}", path.display());
    }
    Ok(())
}

pub fn is_sha256(value: &str) -> bool {
    value.len() == 64 && value.bytes().all(|byte| byte.is_ascii_hexdigit())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn hashes_files() {
        let dir = tempfile::tempdir().unwrap();
        let path = dir.path().join("payload");
        std::fs::write(&path, b"volvid").unwrap();
        let digest = sha256_file(&path).unwrap();
        assert!(is_sha256(&digest));
        verify_sha256(&path, &digest).unwrap();
    }

    #[test]
    fn rejects_malformed_checksums() {
        let dir = tempfile::tempdir().unwrap();
        let path = dir.path().join("payload");
        std::fs::write(&path, b"volvid").unwrap();
        assert!(verify_sha256(&path, "not-a-checksum").is_err());
    }
}
