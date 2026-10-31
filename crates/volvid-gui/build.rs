use std::path::PathBuf;

fn main() {
    println!("cargo:rerun-if-changed=../../assets/icons/volvid.ico");

    if std::env::var("CARGO_CFG_TARGET_OS").as_deref() != Ok("windows") {
        return;
    }

    let manifest_dir = PathBuf::from(
        std::env::var("CARGO_MANIFEST_DIR").expect("CARGO_MANIFEST_DIR is set by Cargo"),
    );
    let icon = manifest_dir.join("../../assets/icons/volvid.ico");
    let icon = icon
        .to_str()
        .expect("the Volvid icon path must be valid UTF-8");

    let mut resource = winresource::WindowsResource::new();
    resource.set_icon(icon);
    resource.set("ProductName", "Volvid");
    resource.set("FileDescription", "Volvid");
    resource.set("InternalName", "Volvid");
    resource.set("OriginalFilename", "Volvid.exe");
    resource.set("LegalCopyright", "Copyright (C) 2026 VolRen");
    resource
        .compile()
        .expect("failed to embed the Windows executable resources");
}
