//! Protocol error types.

use serde::{Deserialize, Serialize};

/// Errors that can occur during protocol encoding/decoding.
#[derive(Debug, thiserror::Error)]
pub enum ProtocolError {
    #[error("unsupported protocol version: {0} (expected {expected})", expected = crate::PROTOCOL_VERSION)]
    UnsupportedVersion(u32),

    #[error("message too large: {size} bytes (max {max})")]
    MessageTooLarge { size: usize, max: usize },

    #[error("code content too large: {size} bytes (max {max})")]
    CodeContentTooLarge { size: usize, max: usize },

    #[error("message text too large: {size} bytes (max {max})")]
    MsgTextTooLarge { size: usize, max: usize },

    #[error("JSON error: {0}")]
    Json(#[from] serde_json::Error),
}

/// Standard error codes sent over the wire.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub enum ErrorCode {
    #[serde(rename = "unsupported_version")]
    UnsupportedVersion,
    #[serde(rename = "unknown_type")]
    UnknownType,
    #[serde(rename = "invalid_payload")]
    InvalidPayload,
    #[serde(rename = "auth_required")]
    AuthRequired,
    #[serde(rename = "auth_failed")]
    AuthFailed,
    #[serde(rename = "capability_missing")]
    CapabilityMissing,
    #[serde(rename = "rate_limited")]
    RateLimited,
    #[serde(rename = "message_too_large")]
    MessageTooLarge,
}

/// Payload for `error` messages.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct ErrorPayload {
    pub ref_id: uuid::Uuid,
    pub code: ErrorCode,
    pub message: String,
}
