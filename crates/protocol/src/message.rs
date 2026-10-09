//! Protocol message types.

use serde::{Deserialize, Serialize};
use uuid::Uuid;

use crate::capability::Capability;
use crate::error::ErrorPayload;

/// All protocol messages, tagged by `type` in JSON.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(tag = "type", content = "payload")]
pub enum Message {
    // --- Session lifecycle ---
    #[serde(rename = "hello")]
    Hello(HelloPayload),

    #[serde(rename = "auth.challenge")]
    AuthChallenge(AuthChallengePayload),

    #[serde(rename = "auth.proof")]
    AuthProof(AuthProofPayload),

    #[serde(rename = "auth.ok")]
    AuthOk(AuthOkPayload),

    #[serde(rename = "heartbeat")]
    Heartbeat(HeartbeatPayload),

    // --- Error ---
    #[serde(rename = "error")]
    Error(ErrorPayload),

    // --- Code sharing ---
    #[serde(rename = "code.share")]
    CodeShare(CodeSharePayload),

    #[serde(rename = "code.ack")]
    CodeAck(CodeAckPayload),

    #[serde(rename = "code.clear")]
    CodeClear(CodeClearPayload),

    // --- Messaging ---
    #[serde(rename = "msg.show")]
    MsgShow(MsgShowPayload),

    #[serde(rename = "msg.ack")]
    MsgAck(MsgAckPayload),
}

// --- Payload structs ---

/// Sent by the agent immediately after connecting.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct HelloPayload {
    pub hostname: String,
    pub username: String,
    pub os: String,
    pub os_version: String,
    pub agent_version: String,
    pub capabilities: Vec<Capability>,
}

/// Console responds to `hello` with a nonce challenge.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct AuthChallengePayload {
    pub ref_id: Uuid,
    pub nonce: String,
}

/// Console proves identity by signing the nonce with Ed25519.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct AuthProofPayload {
    pub ref_id: Uuid,
    pub signature: String,
}

/// Agent confirms authentication succeeded.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct AuthOkPayload {
    pub ref_id: Uuid,
}

/// Bidirectional keepalive.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct HeartbeatPayload {}

/// Share a code snippet with the student.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct CodeSharePayload {
    pub snippet_id: String,
    pub title: String,
    pub lang: String,
    pub content: String,
    pub mode: CodeShareMode,
}

/// How to apply the shared code.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum CodeShareMode {
    #[serde(rename = "replace")]
    Replace,
    #[serde(rename = "append")]
    Append,
}

/// Acknowledges receipt of a code message.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct CodeAckPayload {
    pub ref_id: Uuid,
}

/// Clear the displayed code.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct CodeClearPayload {}

/// Show a message to the student.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct MsgShowPayload {
    pub text: String,
    pub level: MsgLevel,
}

/// Visual level for student-facing messages.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum MsgLevel {
    #[serde(rename = "info")]
    Info,
    #[serde(rename = "warning")]
    Warning,
}

/// Acknowledges the student saw the message.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct MsgAckPayload {
    pub ref_id: Uuid,
}
