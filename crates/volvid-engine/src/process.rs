use std::ffi::OsStr;
use std::process::Stdio;
use std::time::Duration;

use process_wrap::tokio::*;
use thiserror::Error;
use tokio::io::{AsyncBufReadExt, AsyncRead, BufReader};
use tokio::sync::mpsc;
use tokio_util::sync::CancellationToken;

#[derive(Clone, Debug)]
pub struct ProcessLine {
    pub stdout: bool,
    pub text: String,
}

#[derive(Clone, Debug, Default)]
pub struct CaptureOutput {
    pub stdout: Vec<u8>,
    pub stderr: String,
    pub truncated: bool,
    pub success: bool,
    pub code: Option<i32>,
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
    let mut command = CommandWrap::with_new(program, |cmd| {
        cmd.args(args)
            .stdin(Stdio::null())
            .stdout(Stdio::piped())
            .stderr(Stdio::piped());
    });
    #[cfg(unix)]
    command.wrap(ProcessGroup::leader());
    #[cfg(windows)]
    command.wrap(JobObject);
    command.wrap(KillOnDrop);

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
            Ok(0) => break,
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
            Err(_) => break,
        }
    }
}

pub async fn run_capture(
    program: impl AsRef<OsStr>,
    args: &[String],
    timeout: Option<Duration>,
    cancel: Option<CancellationToken>,
    stdout_limit: usize,
    stderr_limit: usize,
) -> Result<CaptureOutput, ProcessError> {
    let mut output = CaptureOutput::default();
    let mut truncated = false;
    let status = run_lines(program, args, timeout, cancel, |line| {
        if line.stdout {
            if output.stdout.len() < stdout_limit {
                let room = stdout_limit - output.stdout.len();
                let bytes = line.text.as_bytes();
                if bytes.len() > room {
                    output.stdout.extend_from_slice(&bytes[..room]);
                    truncated = true;
                } else {
                    output.stdout.extend_from_slice(bytes);
                }
                output.stdout.push(b'\n');
            } else {
                truncated = true;
            }
        } else if output.stderr.len() < stderr_limit {
            let room = stderr_limit - output.stderr.len();
            let text = truncate_utf8(&line.text, room);
            truncated |= text.len() < line.text.len();
            output.stderr.push_str(text);
            output.stderr.push('\n');
        }
    })
    .await?;
    output.truncated = truncated;
    output.success = status.success();
    output.code = status.code();
    if !status.success() {
        // The caller decides how to interpret a non-zero exit; stderr is kept.
    }
    Ok(output)
}

fn truncate_utf8(text: &str, max_bytes: usize) -> &str {
    if text.len() <= max_bytes {
        return text;
    }
    let mut end = max_bytes;
    while end > 0 && !text.is_char_boundary(end) {
        end -= 1;
    }
    &text[..end]
}

pub async fn command_output(
    program: impl AsRef<OsStr>,
    args: &[String],
    timeout: Option<Duration>,
) -> Result<CaptureOutput, ProcessError> {
    run_capture(program, args, timeout, None, 64 << 20, 8 << 10).await
}

pub async fn command_output_cancellable(
    program: impl AsRef<OsStr>,
    args: &[String],
    timeout: Option<Duration>,
    cancel: Option<CancellationToken>,
) -> Result<CaptureOutput, ProcessError> {
    run_capture(program, args, timeout, cancel, 64 << 20, 8 << 10).await
}

#[cfg(test)]
mod tests {
    use super::truncate_utf8;

    #[test]
    fn truncates_utf8_on_char_boundary() {
        let text = "ошибка";
        for limit in 0..=text.len() {
            let truncated = truncate_utf8(text, limit);
            assert!(truncated.len() <= limit);
            assert!(text.starts_with(truncated));
        }
    }
}
