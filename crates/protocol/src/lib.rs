//! Aula protocol: message types and serialization.
//!
//! This crate is I/O-free and OS-free by design.
//! It depends only on `serde`, `serde_json`, `thiserror`, and `uuid`.

mod capability;
mod envelope;
mod error;
mod message;

pub use capability::Capability;
pub use envelope::{decode, encode, Envelope, PROTOCOL_VERSION};
pub use error::{ErrorCode, ErrorPayload, ProtocolError};
pub use message::{
    AuthChallengePayload, AuthOkPayload, AuthProofPayload, CodeAckPayload, CodeClearPayload,
    CodeShareMode, CodeSharePayload, HeartbeatPayload, HelloPayload, Message, MsgAckPayload,
    MsgLevel, MsgShowPayload,
};

/// Maximum message size in bytes (1 MiB).
pub const MAX_MESSAGE_SIZE: usize = 1_048_576;

/// Maximum code content size in bytes (512 KiB).
pub const MAX_CODE_CONTENT_SIZE: usize = 512 * 1024;

/// Maximum message text size in bytes (4 KiB).
pub const MAX_MSG_TEXT_SIZE: usize = 4 * 1024;
