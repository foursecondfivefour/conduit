//! End-to-end check of the X11 client against the fake server in
//! `tools/fake_x_server.py`.
//!
//! There is no X server in CI, so the client's wire protocol is exercised
//! against a scripted server. The test is skipped when Python is unavailable.

use std::path::PathBuf;
use std::process::{Command, Stdio};

fn python() -> Option<&'static str> {
    ["python3", "python"].into_iter().find(|candidate| {
        Command::new(candidate)
            .arg("--version")
            .stdout(Stdio::null())
            .stderr(Stdio::null())
            .status()
            .is_ok_and(|status| status.success())
    })
}

#[test]
fn client_speaks_x11_to_a_fake_server() {
    let Some(python) = python() else {
        eprintln!("skipping: no python interpreter");
        return;
    };
    let tools = PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("tools/fake_x_server.py");
    let socket = PathBuf::from("/tmp/.X11-unix/X77");
    let report = PathBuf::from("/tmp/conduit-fake-x-integration.json");
    let _ = std::fs::remove_file(&report);

    let mut server = Command::new(python)
        .arg(&tools)
        .env("FAKE_X_SOCKET", &socket)
        .env("FAKE_X_REPORT", &report)
        .stdout(Stdio::null())
        .stderr(Stdio::null())
        .spawn()
        .expect("start fake X server");

    // Wait for the socket to appear.
    for _ in 0..100 {
        if socket.exists() {
            break;
        }
        std::thread::sleep(std::time::Duration::from_millis(50));
    }
    assert!(socket.exists(), "fake server did not start");

    let status = Command::new(env!("CARGO_BIN_EXE_conduit-tui"))
        .env("DISPLAY", ":77")
        .status()
        .expect("run the client");
    let server_status = server.wait().expect("server exit");

    assert!(server_status.success(), "fake server reported a problem");
    let json = std::fs::read_to_string(&report).expect("report written");
    assert!(status.success(), "client exited with {status}\n{json}");
    assert!(json.contains("\"ok\": true"), "report: {json}");
    assert!(json.contains("\"close_requested\": true"), "report: {json}");
    for expected in ["WM_NAME", "_NET_WM_NAME", "UTF8_STRING"] {
        assert!(json.contains(expected), "report lacks {expected}: {json}");
    }
}
