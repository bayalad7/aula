//! Envelope wrapping all protocol messages, plus encode/decode with validation.

use serde::{Deserialize, Serialize};
use uuid::Uuid;

use crate::error::ProtocolError;
use crate::message::Message;
use crate::{MAX_CODE_CONTENT_SIZE, MAX_MESSAGE_SIZE, MAX_MSG_TEXT_SIZE};

/// Current protocol version.
pub const PROTOCOL_VERSION: u32 = 1;

/// Top-level wire format for all messages.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct Envelope {
    pub v: u32,
    pub id: Uuid,
    pub ts: u64,
    #[serde(flatten)]
    pub message: Message,
}

impl Envelope {
    /// Create a new envelope with a random UUID and the current protocol version.
    /// The caller provides the timestamp and message.
    pub fn new(ts: u64, message: Message) -> Self {
        Self {
            v: PROTOCOL_VERSION,
            id: Uuid::new_v4(),
            ts,
            message,
        }
    }
}

/// Serialize an envelope to a JSON string, validating size limits.
pub fn encode(envelope: &Envelope) -> Result<String, ProtocolError> {
    validate_payload_limits(&envelope.message)?;

    let json = serde_json::to_string(envelope)?;

    if json.len() > MAX_MESSAGE_SIZE {
        return Err(ProtocolError::MessageTooLarge {
            size: json.len(),
            max: MAX_MESSAGE_SIZE,
        });
    }

    Ok(json)
}

/// Deserialize a JSON string into an envelope, validating version and size limits.
pub fn decode(json: &str) -> Result<Envelope, ProtocolError> {
    if json.len() > MAX_MESSAGE_SIZE {
        return Err(ProtocolError::MessageTooLarge {
            size: json.len(),
            max: MAX_MESSAGE_SIZE,
        });
    }

    // First pass: check version before full deserialization.
    let raw: serde_json::Value = serde_json::from_str(json)?;
    if let Some(v) = raw.get("v").and_then(|v| v.as_u64()) {
        let v = v as u32;
        if v != PROTOCOL_VERSION {
            return Err(ProtocolError::UnsupportedVersion(v));
        }
    }

    let envelope: Envelope = serde_json::from_value(raw)?;
    validate_payload_limits(&envelope.message)?;

    Ok(envelope)
}

/// Check content size limits on payloads that carry large data.
fn validate_payload_limits(message: &Message) -> Result<(), ProtocolError> {
    match message {
        Message::CodeShare(payload) if payload.content.len() > MAX_CODE_CONTENT_SIZE => {
            return Err(ProtocolError::CodeContentTooLarge {
                size: payload.content.len(),
                max: MAX_CODE_CONTENT_SIZE,
            });
        }
        Message::MsgShow(payload) if payload.text.len() > MAX_MSG_TEXT_SIZE => {
            return Err(ProtocolError::MsgTextTooLarge {
                size: payload.text.len(),
                max: MAX_MSG_TEXT_SIZE,
            });
        }
        _ => {}
    }
    Ok(())
}
