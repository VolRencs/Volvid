use std::ffi::OsStr;
use std::process::Stdio;
use std::time::Duration;

use anyhow::anyhow;
use process_wrap::tokio::*;
use thiserror::Error;
use tokio::io::{AsyncBufReadExt, AsyncRead, BufReader};
use tokio::sync::mpsc;
use tokio_util::sync::CancellationToken;
use volvid_core::truncate_utf8;
#[cfg(windows)]
use windows::Win32::System::Threading::CREATE_NO_WINDOW;

const STDOUT_LIMIT: usize = 64 << 20;
const STDERR_LIMIT: usize = 8 << 10;

#[derive(Clone, Debug)]
pub struct ProcessLine {
    pub stdout: bool,
    pub text: String,
}

#[derive(Clone, Debug, Default)]
pub struct CaptureOutput {
    pub stdout: Vec<u8>,
    pub stderr: String,
    pub success: bool,
}

#[derive(Debug, Error)]
pub enum ProcessError {
    #[error("failed to start {program}: {source}")]
    Spawn {
        program: String,
        #[source]
        source: std::io::Error,
    },
    #[error("process I/O error: {0}")]
    Io(#[from] std::io::Error),
    #[error("operation cancelled")]
    Cancelled,
    #[error("operation timed out")]
    TimedOut,
}

impl ProcessError {
    pub fn into_anyhow(self) -> anyhow::Error {
        match self {
            Self::Cancelled => anyhow!("operation cancelled"),
            Self::TimedOut => anyhow!("operation timed out"),
            other => other.into(),
        }
    }
}

/// Hides the console window that Windows otherwise creates for console tools.
pub(crate) trait HideConsole {
    fn hide_console(&mut self) -> &mut Self;
}

impl HideConsole for std::process::Command {
    fn hide_console(&mut self) -> &mut Self {
        #[cfg(windows)]
        {
            use std::os::windows::process::CommandExt;
            self.creation_flags(CREATE_NO_WINDOW.0);
        }
        self
    }
}

impl HideConsole for CommandWrap {
    fn hide_console(&mut self) -> &mut Self {
        #[cfg(windows)]
        self.wrap(process_wrap::tokio::CreationFlags(CREATE_NO_WINDOW));
        self
    }
}

/// Creates a regular command with the shared Windows console policy applied.
#[allow(clippy::disallowed_methods)]
pub(crate) fn std_command(program: impl AsRef<OsStr>) -> std::process::Command {
    let mut command = std::process::Command::new(program);
    command.hide_console();
    command
}

#[allow(clippy::disallowed_methods)]
fn command_wrap(program: impl AsRef<OsStr>, args: &[String]) -> CommandWrap {
    let mut command = CommandWrap::with_new(program, |cmd| {
        cmd.args(args)
            .stdin(Stdio::null())
            .stdout(Stdio::piped())
            .stderr(Stdio::piped())
            // Python tools such as yt-dlp must use UTF-8 even on a Windows
            // console code page; native tools simply ignore these variables.
            .env("PYTHONUTF8", "1")
            .env("PYTHONIOENCODING", "utf-8");
    });
    command.hide_console();
    #[cfg(unix)]
    command.wrap(ProcessGroup::leader());
    #[cfg(windows)]
    {
        // JobObject + KillOnDrop also terminates the whole child tree when the
        // download is cancelled or the application exits.
        command.wrap(JobObject);
    }
    command.wrap(KillOnDrop);
    command
}

pub async fn run_lines<F>(
    program: impl AsRef<OsStr>,
    args: &[String],
    timeout: Option<Duration>,
    cancel: Option<CancellationToken>,
    mut on_line: F,
) -> Result<std::process::ExitStatus, ProcessError>
where
    F: FnMut(ProcessLine),
{
    let program_display = program.as_ref().to_string_lossy().into_owned();
    let mut command = command_wrap(program, args);

    let mut child = command.spawn().map_err(|source| ProcessError::Spawn {
        program: program_display,
        source,
    })?;

    let stdout = child.stdout().take();
    let stderr = child.stderr().take();
    let (tx, mut rx) = mpsc::unbounded_channel::<ProcessLine>();
    if let Some(stdout) = stdout {
        tokio::spawn(read_lines(stdout, true, tx.clone()));
    }
    if let Some(stderr) = stderr {
        tokio::spawn(read_lines(stderr, false, tx.clone()));
    }
    drop(tx);

    let timeout_future = async {
        match timeout {
            Some(duration) => tokio::time::sleep(duration).await,
            None => std::future::pending().await,
        }
    };
    tokio::pin!(timeout_future);
    let cancel_future = async {
        match cancel.clone() {
            Some(token) => token.cancelled().await,
            None => std::future::pending().await,
        }
    };
    tokio::pin!(cancel_future);

    let mut channel_closed = false;
    let mut status = None;
    while status.is_none() {
        tokio::select! {
            _ = &mut cancel_future => {
                let _ = Box::into_pin(child.kill()).await;
                return Err(ProcessError::Cancelled);
            }
            _ = &mut timeout_future => {
                let _ = Box::into_pin(child.kill()).await;
                return Err(ProcessError::TimedOut);
            }
            line = rx.recv(), if !channel_closed => {
                match line {
                    Some(line) => on_line(line),
                    None => channel_closed = true,
                }
            }
            result = child.wait() => {
                status = Some(result?);
            }
        }
    }

    // Drain output that was produced just before the process exited. The
    // process can exit before the reader tasks have forwarded the last lines.
    while let Ok(Some(line)) = tokio::time::timeout(Duration::from_secs(2), rx.recv()).await {
        on_line(line);
    }
    Ok(status.expect("status set in loop"))
}

async fn read_lines<R>(reader: R, stdout: bool, tx: mpsc::UnboundedSender<ProcessLine>)
where
    R: AsyncRead + Unpin,
{
    let mut reader = BufReader::new(reader);
    let mut buffer = Vec::new();
    loop {
        buffer.clear();
        match reader.read_until(b'\n', &mut buffer).await {
            Ok(0) | Err(_) => break,
            Ok(_) => {
                while buffer
                    .last()
                    .is_some_and(|byte| *byte == b'\n' || *byte == b'\r')
                {
                    buffer.pop();
                }
                let text = String::from_utf8_lossy(&buffer).trim().to_owned();
                if !text.is_empty() && tx.send(ProcessLine { stdout, text }).is_err() {
                    break;
                }
            }
        }
    }
}

pub async fn command_output(
    program: impl AsRef<OsStr>,
    args: &[String],
    timeout: Option<Duration>,
    cancel: Option<CancellationToken>,
) -> Result<CaptureOutput, ProcessError> {
    let mut output = CaptureOutput::default();
    let status = run_lines(program, args, timeout, cancel, |line| {
        if line.stdout {
            if output.stdout.len() < STDOUT_LIMIT {
                let room = STDOUT_LIMIT - output.stdout.len();
                let bytes = line.text.as_bytes();
                output
                    .stdout
                    .extend_from_slice(&bytes[..bytes.len().min(room)]);
                output.stdout.push(b'\n');
            }
        } else if output.stderr.len() < STDERR_LIMIT {
            let room = STDERR_LIMIT - output.stderr.len();
            output.stderr.push_str(truncate_utf8(&line.text, room));
            output.stderr.push('\n');
        }
    })
    .await?;
    output.success = status.success();
    Ok(output)
}
