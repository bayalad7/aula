//! Core-level error types.

use aula_protocol::ProtocolError;

/// Errors originating from the core crate.
#[derive(Debug, thiserror::Error)]
pub enum CoreError {
    #[error("I/O error: {0}")]
    Io(#[from] std::io::Error),

    #[error("Protocol error: {0}")]
    Protocol(#[from] ProtocolError),

    #[error("WebSocket error: {0}")]
    WebSocket(Box<tokio_tungstenite::tungstenite::Error>),

    #[error("JSON error: {0}")]
    Json(#[from] serde_json::Error),

    #[error("Connection closed")]
    ConnectionClosed,

    #[error("Heartbeat timeout")]
    HeartbeatTimeout,

    #[error("Unexpected message: expected {expected}, got {got}")]
    UnexpectedMessage { expected: String, got: String },
}

impl From<tokio_tungstenite::tungstenite::Error> for CoreError {
    fn from(e: tokio_tungstenite::tungstenite::Error) -> Self {
        CoreError::WebSocket(Box::new(e))
    }
}
