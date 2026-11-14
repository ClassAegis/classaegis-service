// SPDX-License-Identifier: AGPL-3.0-only
#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

use std::{fs, io::{Read, Write}, net::TcpStream, path::PathBuf, process::Command, time::Duration};
use serde_json::Value;
use tauri::{
    image::Image,
    menu::{Menu, MenuItem, PredefinedMenuItem},
    tray::{MouseButton, MouseButtonState, TrayIconBuilder, TrayIconEvent},
    AppHandle, Manager, WebviewWindow,
};

const APP_NAME: &str = "ClassAegisService";
const CORE_PORT: u16 = 39817;

fn logs_dir() -> PathBuf {
    PathBuf::from(std::env::var_os("ProgramData").unwrap_or_else(|| ".".into()))
        .join("oneclassmesh")
        .join("logs")
}

fn config_path(app: &AppHandle) -> String {
    if let Some(path) = std::env::var_os("OCM_CONFIG").map(PathBuf::from).filter(|path| path.is_file()) {
        return path.display().to_string();
    }
    app.path()
        .executable_dir()
        .ok()
        .map(|root| root.join("conf").join("config.toml"))
        .filter(|path| path.is_file())
        .map(|path| path.display().to_string())
        .unwrap_or_default()
}

fn config_value(content: &str, key: &str) -> Option<String> {
    content.lines().find_map(|line| {
        let (name, value) = line.trim().split_once('=')?;
        (name.trim() == key).then(|| value.trim().trim_matches('"').to_string())
    })
}

#[tauri::command]
fn get_status(app: AppHandle) -> Result<Value, String> {
    let config = config_path(&app);
    if config.is_empty() { return Err("未找到 conf/config.toml".to_string()); }
    let content = fs::read_to_string(config).map_err(|error| error.to_string())?;
    let token = config_value(&content, "ipc_token")
        .filter(|token| !token.is_empty())
        .or_else(|| config_value(&content, "mqtt_token").filter(|token| !token.is_empty()).map(|token| format!("ocm-{token}")))
        .ok_or_else(|| "配置中未设置 ipc_token".to_string())?;
    let mut stream = TcpStream::connect(("127.0.0.1", CORE_PORT)).map_err(|error| error.to_string())?;
    stream.set_read_timeout(Some(Duration::from_secs(3))).map_err(|error| error.to_string())?;
    write!(stream, "GET /status HTTP/1.1\r\nHost: 127.0.0.1:{CORE_PORT}\r\nX-Ocm-Token: {token}\r\nConnection: close\r\n\r\n").map_err(|error| error.to_string())?;
    let mut response = String::new();
    stream.read_to_string(&mut response).map_err(|error| error.to_string())?;
    let (headers, body) = response.split_once("\r\n\r\n").ok_or_else(|| "Core 返回格式错误".to_string())?;
    if !headers.starts_with("HTTP/1.1 200") && !headers.starts_with("HTTP/1.0 200") {
        return Err(headers.lines().next().unwrap_or("Core 请求失败").to_string());
    }
    let data: Value = serde_json::from_str(body).map_err(|error| error.to_string())?;
    Ok(serde_json::json!({ "ok": true, "data": data }))
}

#[tauri::command]
fn get_app_info(app: AppHandle) -> Value {
    serde_json::json!({
        "name": APP_NAME,
        "version": env!("CARGO_PKG_VERSION"),
        "protocol": "0.1",
        "configPath": config_path(&app)
    })
}

#[tauri::command]
fn window_minimize(window: WebviewWindow) -> Result<(), String> {
    window.minimize().map_err(|error| error.to_string())
}

#[tauri::command]
fn window_close(window: WebviewWindow) -> Result<(), String> {
    window.hide().map_err(|error| error.to_string())
}

#[tauri::command]
fn open_logs() -> Result<(), String> {
    let directory = logs_dir();
    fs::create_dir_all(&directory).map_err(|error| error.to_string())?;
    Command::new("explorer.exe").arg(directory).spawn().map(|_| ()).map_err(|error| error.to_string())
}

#[tauri::command]
fn list_logs() -> Value {
    let mut files = Vec::new();
    if let Ok(entries) = fs::read_dir(logs_dir()) {
        for entry in entries.flatten() {
            let path = entry.path();
            let Some(name) = path.file_name().and_then(|name| name.to_str()) else { continue };
            if !name.ends_with(".log") && !name.contains(".log.") { continue }
            if let Ok(metadata) = entry.metadata() {
                files.push(serde_json::json!({ "name": name, "backup": name.contains(".log."), "size": metadata.len() }));
            }
        }
    }
    serde_json::json!({ "ok": true, "files": files })
}

#[tauri::command]
fn read_log(name: String) -> Value {
    if name.len() > 180 || !name.chars().all(|ch| ch.is_ascii_alphanumeric() || "_.-".contains(ch)) {
        return serde_json::json!({ "ok": false, "error": "非法文件名" });
    }
    match fs::read_to_string(logs_dir().join(&name)) {
        Ok(text) => serde_json::json!({ "ok": true, "text": text, "size": text.len() }),
        Err(error) => serde_json::json!({ "ok": false, "error": error.to_string() }),
    }
}

fn show_main(app: &AppHandle) {
    if let Some(window) = app.get_webview_window("main") {
        let _ = window.unminimize();
        let _ = window.show();
        let _ = window.set_focus();
    }
}

fn main() {
    tauri::Builder::default()
        .plugin(tauri_plugin_single_instance::init(|app, _args, _cwd| show_main(app)))
        .setup(|app| {
            let main_window = app.get_webview_window("main").ok_or("main window is missing")?;
            let main_window_for_close = main_window.clone();
            main_window.on_window_event(move |event| {
                if matches!(event, tauri::WindowEvent::Resized(_)) {
                    if main_window_for_close.is_fullscreen().unwrap_or(false) {
                        let _ = main_window_for_close.set_fullscreen(false);
                    }
                    if main_window_for_close.is_maximized().unwrap_or(false) {
                        let _ = main_window_for_close.unmaximize();
                    }
                }
                if let tauri::WindowEvent::CloseRequested { api, .. } = event {
                    api.prevent_close();
                    let _ = main_window_for_close.hide();
                }
            });
            let show = MenuItem::with_id(app, "show", "显示 ClassAegisService", true, None::<&str>)?;
            let separator = PredefinedMenuItem::separator(app)?;
            let quit = MenuItem::with_id(app, "quit", "退出", true, None::<&str>)?;
            let menu = Menu::with_items(app, &[&show, &separator, &quit])?;
            let icon_bytes = include_bytes!("../../build/icons/32x32.png");
            let icon_bitmap = image::load_from_memory(icon_bytes)?.to_rgba8();
            let (icon_width, icon_height) = icon_bitmap.dimensions();
            let icon = Image::new_owned(icon_bitmap.into_raw(), icon_width, icon_height);
            TrayIconBuilder::new()
                .icon(icon)
                .tooltip("ClassAegisService · 后台运行中")
                .menu(&menu)
                .on_menu_event(|app, event| match event.id().as_ref() {
                    "show" => show_main(app),
                    "quit" => app.exit(0),
                    _ => {}
                })
                .on_tray_icon_event(|tray, event| {
                    if let TrayIconEvent::Click { button: MouseButton::Left, button_state: MouseButtonState::Up, .. } = event {
                        show_main(tray.app_handle());
                    }
                })
                .build(app)?;
            Ok(())
        })
        .invoke_handler(tauri::generate_handler![get_status, get_app_info, window_minimize, window_close, open_logs, list_logs, read_log])
        .run(tauri::generate_context!())
        .expect("failed to run ClassAegisService");
}
