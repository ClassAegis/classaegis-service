// SPDX-License-Identifier: AGPL-3.0-only
fn main() {
    // A Windows drive path can deserialize as a URL, which disables asset embedding.
    // Refuse release packages that would depend on the build machine at runtime.
    if std::env::var("PROFILE").as_deref() == Ok("release") {
        let config: serde_json::Value = serde_json::from_str(
            &std::fs::read_to_string("tauri.conf.json").expect("read Tauri config"),
        ).expect("parse Tauri config");
        let override_config: serde_json::Value = std::env::var("TAURI_CONFIG")
            .ok()
            .map(|value| serde_json::from_str(&value).expect("parse Tauri override"))
            .unwrap_or_default();
        let dist = override_config.pointer("/build/frontendDist")
            .unwrap_or(&config["build"]["frontendDist"])
            .as_str().expect("frontendDist must be a relative asset directory");
        let path = std::path::Path::new(dist);
        assert!(!path.is_absolute() && !dist.contains(':') && !dist.starts_with('\\'),
            "frontendDist must be a relative asset directory; URLs and absolute paths skip asset embedding");
        assert!(path.join("index.html").is_file(), "frontendDist/index.html is missing; build the frontend before packaging");
    }
    tauri_build::build()
}
