//! Minimal Conduit window, written against `conduit-tui`.
//!
//! Modes:
//!
//! * default — open an X11 window and run the event loop;
//! * `--dump-ppm PATH` — render one frame to a PPM file and exit, for CI and for
//!   machines without a display;
//! * `--dump-ascii` — render one frame as text and exit.
//!
//! Window size can be overridden with `--size WIDTHxHEIGHT`.

#![forbid(unsafe_code)]

use std::process::ExitCode;

use conduit_tui::app::App;

/// Parsed command line.
#[derive(Debug, Default, PartialEq, Eq)]
struct Args {
    dump_ppm: Option<String>,
    dump_ascii: bool,
    width: u16,
    height: u16,
}

const DEFAULT_WIDTH: u16 = 420;
const DEFAULT_HEIGHT: u16 = 560;

fn parse_args(argv: &[String]) -> Result<Args, String> {
    let mut args = Args {
        dump_ppm: None,
        dump_ascii: false,
        width: DEFAULT_WIDTH,
        height: DEFAULT_HEIGHT,
    };
    let mut index = 0;
    while index < argv.len() {
        match argv[index].as_str() {
            "--dump-ppm" => {
                index += 1;
                let path = argv
                    .get(index)
                    .ok_or_else(|| "--dump-ppm needs a path".to_owned())?;
                args.dump_ppm = Some(path.clone());
            }
            "--dump-ascii" => args.dump_ascii = true,
            "--size" => {
                index += 1;
                let value = argv
                    .get(index)
                    .ok_or_else(|| "--size needs WIDTHxHEIGHT".to_owned())?;
                let (width, height) = value
                    .split_once(['x', 'X'])
                    .ok_or_else(|| format!("--size wants WIDTHxHEIGHT, got {value:?}"))?;
                args.width = width
                    .parse()
                    .map_err(|_| format!("bad width in --size {value:?}"))?;
                args.height = height
                    .parse()
                    .map_err(|_| format!("bad height in --size {value:?}"))?;
            }
            "--help" | "-h" => {
                return Err(String::new());
            }
            other => return Err(format!("unknown argument {other:?}")),
        }
        index += 1;
    }
    Ok(args)
}

fn usage() -> String {
    String::from(
        "conduit-tui — Conduit's own data-described GUI\n\
         \n\
         usage: conduit-tui [--size WIDTHxHEIGHT] [--dump-ppm PATH | --dump-ascii]\n\
         \n\
         Without --dump-ppm or --dump-ascii, opens an X11 window using $DISPLAY.",
    )
}

fn main() -> ExitCode {
    let argv: Vec<String> = std::env::args().skip(1).collect();
    let args = match parse_args(&argv) {
        Ok(args) => args,
        Err(message) if message.is_empty() => {
            println!("{}", usage());
            return ExitCode::SUCCESS;
        }
        Err(message) => {
            eprintln!("conduit-tui: {message}\n\n{}", usage());
            return ExitCode::from(2);
        }
    };

    let app = App::new(args.width, args.height);

    if let Some(path) = &args.dump_ppm {
        let canvas = app.canvas();
        if let Err(err) = std::fs::write(path, canvas.to_ppm()) {
            eprintln!("conduit-tui: cannot write {path}: {err}");
            return ExitCode::FAILURE;
        }
        println!("wrote {path} ({}x{})", canvas.width(), canvas.height());
        return ExitCode::SUCCESS;
    }

    if args.dump_ascii {
        print!("{}", app.canvas().to_ascii());
        return ExitCode::SUCCESS;
    }

    match conduit_tui::app::run() {
        Ok(()) => ExitCode::SUCCESS,
        Err(err) => {
            eprintln!("conduit-tui: {err}");
            eprintln!("hint: set DISPLAY, or use --dump-ppm for a headless frame");
            ExitCode::FAILURE
        }
    }
}

#[cfg(test)]
mod tests {
    use super::{parse_args, Args, DEFAULT_HEIGHT, DEFAULT_WIDTH};

    fn args(list: &[&str]) -> Vec<String> {
        list.iter().map(|s| (*s).to_owned()).collect()
    }

    #[test]
    fn empty_command_line_uses_defaults() {
        let parsed = parse_args(&[]).expect("defaults");
        assert_eq!(
            parsed,
            Args {
                dump_ppm: None,
                dump_ascii: false,
                width: DEFAULT_WIDTH,
                height: DEFAULT_HEIGHT,
            }
        );
    }

    #[test]
    fn parses_dump_ppm() {
        let parsed = parse_args(&args(&["--dump-ppm", "out.ppm"])).expect("parsed");
        assert_eq!(parsed.dump_ppm.as_deref(), Some("out.ppm"));
    }

    #[test]
    fn parses_size() {
        let parsed = parse_args(&args(&["--size", "800x600"])).expect("parsed");
        assert_eq!((parsed.width, parsed.height), (800, 600));
    }

    #[test]
    fn rejects_bad_size() {
        assert!(parse_args(&args(&["--size", "800"])).is_err());
        assert!(parse_args(&args(&["--size", "axb"])).is_err());
        assert!(parse_args(&args(&["--size"])).is_err());
    }

    #[test]
    fn rejects_unknown_flags() {
        assert!(parse_args(&args(&["--nope"])).is_err());
    }

    #[test]
    fn help_is_a_successful_noop() {
        assert_eq!(parse_args(&args(&["--help"])), Err(String::new()));
    }
}
