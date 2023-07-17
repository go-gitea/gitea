---
display_name: Tauri
github_url: https://github.com/tauri-apps/tauri
logo: tauri.png
short_description: Tauri is a framework for building tiny, blazingly fast binaries for all major desktop platforms.
topic: tauri
url: https://tauri.app/
---

Tauri is a framework for building tiny, blazingly fast binaries for all major desktop platforms. Developers can integrate any front-end framework that compiles to HTML, JS and CSS for building their user interface. The backend of the application is a rust-sourced binary with an API that the front-end can interact with.

The user interface in Tauri apps currently leverages tao as a window handling library on macOS and Windows, and gtk on Linux via the Tauri-team incubated and maintained WRY, which creates a unified interface to the system webview (and other goodies like Menu and Taskbar), leveraging WebKit on macOS, WebView2 on Windows and WebKitGTK on Linux.