//! WebSocket server for the teacher's console.
//!
//! Accepts agent connections over TLS, authenticates with Ed25519, processes
//! `hello` messages, maintains a session registry, and monitors heartbeats.

use std::collections::HashMap;
use std::net::SocketAddr;
use std::sync::Arc;
use std::time::{Duration, Instant};

use aula_protocol::{
    decode, encode, AuthChallengePayload, AuthProofPayload, Capability, Envelope, HeartbeatPayload,
    Message,
};
use ed25519_dalek::SigningKey;
use futures_util::{SinkExt, StreamExt};
use tokio::net::{TcpListener, TcpStream};
use tokio::sync::{mpsc, RwLock};
use tokio_tungstenite::tungstenite;
use uuid::Uuid;

use crate::auth;
use crate::error::CoreError;
use crate::tls::TlsIdentity;

/// How often the server checks for stale sessions.
const REAP_INTERVAL: Duration = Duration::from_secs(5);

/// A session is considered dead if no message arrives within this window.
const HEARTBEAT_TIMEOUT: Duration = Duration::from_secs(15);

/// Interval for sending heartbeats to agents.
const HEARTBEAT_INTERVAL: Duration = Duration::from_secs(5);

/// Information about a connected agent session.
#[derive(Debug, Clone)]
pub struct SessionInfo {
    pub id: Uuid,
    pub hostname: String,
    pub username: String,
    pub os: String,
    pub os_version: String,
    pub agent_version: String,
    pub capabilities: Vec<Capability>,
    pub addr: SocketAddr,
    pub connected: bool,
    pub last_seen: Instant,
}

/// Event emitted by the server when sessions change.
#[derive(Debug, Clone)]
pub enum SessionEvent {
    Connected(SessionInfo),
    Disconnected(Uuid),
}

/// Thread-safe registry of active agent sessions.
#[derive(Debug, Clone, Default)]
pub struct SessionRegistry {
    sessions: Arc<RwLock<HashMap<Uuid, SessionInfo>>>,
}

impl SessionRegistry {
    pub fn new() -> Self {
        Self::default()
    }

    pub async fn insert(&self, info: SessionInfo) {
        self.sessions.write().await.insert(info.id, info);
    }

    pub async fn remove(&self, id: &Uuid) -> Option<SessionInfo> {
        self.sessions.write().await.remove(id)
    }

    pub async fn mark_disconnected(&self, id: &Uuid) {
        if let Some(info) = self.sessions.write().await.get_mut(id) {
            info.connected = false;
        }
    }

    pub async fn update_last_seen(&self, id: &Uuid) {
        if let Some(info) = self.sessions.write().await.get_mut(id) {
            info.last_seen = Instant::now();
        }
    }

    /// Returns a snapshot of all sessions.
    pub async fn snapshot(&self) -> Vec<SessionInfo> {
        self.sessions.read().await.values().cloned().collect()
    }

    /// Returns the number of connected sessions.
    pub async fn connected_count(&self) -> usize {
        self.sessions
            .read()
            .await
            .values()
            .filter(|s| s.connected)
            .count()
    }

    /// Reap sessions that haven't been seen within the heartbeat timeout.
    /// Returns the IDs of sessions that were marked disconnected.
    async fn reap_stale(&self) -> Vec<Uuid> {
        let now = Instant::now();
        let mut stale = Vec::new();
        let mut sessions = self.sessions.write().await;
        for (id, info) in sessions.iter_mut() {
            if info.connected && now.duration_since(info.last_seen) > HEARTBEAT_TIMEOUT {
                info.connected = false;
                stale.push(*id);
                tracing::info!(session_id = %id, hostname = %info.hostname, "Session timed out");
            }
        }
        stale
    }
}

/// Configuration for the WebSocket server.
pub struct ServerConfig {
    /// Address to bind the TCP listener.
    pub bind_addr: SocketAddr,
    /// TLS identity (certificate + key) for the server.
    pub tls_identity: TlsIdentity,
    /// Ed25519 signing key for authentication.
    pub signing_key: SigningKey,
}

/// The WebSocket server that accepts agent connections.
pub struct Server {
    config: Arc<ServerInner>,
    registry: SessionRegistry,
}

/// Shared server state (behind `Arc` for spawned tasks).
struct ServerInner {
    bind_addr: SocketAddr,
    tls_acceptor: tokio_rustls::TlsAcceptor,
    signing_key: SigningKey,
}

impl Server {
    pub fn new(config: ServerConfig, registry: SessionRegistry) -> Result<Self, CoreError> {
        let tls_config = crate::tls::server_config(&config.tls_identity)?;
        let tls_acceptor = tokio_rustls::TlsAcceptor::from(tls_config);

        Ok(Self {
            config: Arc::new(ServerInner {
                bind_addr: config.bind_addr,
                tls_acceptor,
                signing_key: config.signing_key,
            }),
            registry,
        })
    }

    pub fn registry(&self) -> &SessionRegistry {
        &self.registry
    }

    /// Run the server until cancelled.
    ///
    /// Returns a channel that emits session events.
    pub async fn run(
        &self,
        cancel: tokio_util::sync::CancellationToken,
    ) -> Result<mpsc::Receiver<SessionEvent>, CoreError> {
        let listener = TcpListener::bind(self.config.bind_addr).await?;
        let local_addr = listener.local_addr()?;

        tracing::info!(addr = %local_addr, "Server listening (TLS)");

        let (event_tx, event_rx) = mpsc::channel(64);

        // Spawn the reaper task.
        let registry = self.registry.clone();
        let reaper_cancel = cancel.clone();
        let reaper_tx = event_tx.clone();
        tokio::spawn(async move {
            loop {
                tokio::select! {
                    _ = reaper_cancel.cancelled() => return,
                    _ = tokio::time::sleep(REAP_INTERVAL) => {
                        let stale = registry.reap_stale().await;
                        for id in stale {
                            let _ = reaper_tx.send(SessionEvent::Disconnected(id)).await;
                        }
                    }
                }
            }
        });

        // Spawn the accept loop.
        let inner = self.config.clone();
        let registry = self.registry.clone();
        tokio::spawn(async move {
            loop {
                tokio::select! {
                    _ = cancel.cancelled() => {
                        tracing::info!("Server shutting down");
                        return;
                    }
                    result = listener.accept() => {
                        match result {
                            Ok((stream, addr)) => {
                                tracing::info!(peer = %addr, "Accepted TCP connection");
                                let inner = inner.clone();
                                let registry = registry.clone();
                                let event_tx = event_tx.clone();
                                let conn_cancel = cancel.clone();
                                tokio::spawn(async move {
                                    if let Err(e) = handle_connection(
                                        stream, addr, &inner, registry, event_tx, conn_cancel,
                                    ).await {
                                        tracing::warn!(peer = %addr, error = %e, "Connection error");
                                    }
                                });
                            }
                            Err(e) => {
                                tracing::warn!(error = %e, "Failed to accept connection");
                            }
                        }
                    }
                }
            }
        });

        Ok(event_rx)
    }
}

/// Handle a single agent connection: TLS → WebSocket → hello → auth → heartbeat loop.
async fn handle_connection(
    stream: TcpStream,
    addr: SocketAddr,
    inner: &ServerInner,
    registry: SessionRegistry,
    event_tx: mpsc::Sender<SessionEvent>,
    cancel: tokio_util::sync::CancellationToken,
) -> Result<(), CoreError> {
    // --- TLS handshake ---
    let tls_stream = inner
        .tls_acceptor
        .accept(stream)
        .await
        .map_err(|e| CoreError::Tls(format!("TLS accept failed: {e}")))?;

    tracing::info!(peer = %addr, "TLS handshake complete");

    // --- WebSocket handshake over TLS ---
    let ws_stream = tokio_tungstenite::accept_async(tls_stream).await?;
    let (mut ws_sink, mut ws_stream_rx) = ws_stream.split();

    // --- Wait for the hello message ---
    let (hello_id, hello) = loop {
        tokio::select! {
            _ = cancel.cancelled() => return Ok(()),
            msg = ws_stream_rx.next() => {
                match msg {
                    Some(Ok(tungstenite::Message::Text(text))) => {
                        let envelope = decode(&text)?;
                        match envelope.message {
                            Message::Hello(hello) => {
                                tracing::info!(
                                    peer = %addr,
                                    hostname = %hello.hostname,
                                    msg_type = "hello",
                                    "Received hello"
                                );
                                break (envelope.id, hello);
                            }
                            other => {
                                let got = format!("{:?}", std::mem::discriminant(&other));
                                return Err(CoreError::UnexpectedMessage {
                                    expected: "hello".to_string(),
                                    got,
                                });
                            }
                        }
                    }
                    Some(Ok(tungstenite::Message::Close(_))) | None => {
                        return Err(CoreError::ConnectionClosed);
                    }
                    Some(Ok(_)) => continue,
                    Some(Err(e)) => return Err(CoreError::from(e)),
                }
            }
        }
    };

    // --- Authentication: send auth.challenge + auth.proof ---

    // Generate nonce and send auth.challenge.
    let nonce = auth::generate_nonce();
    let nonce_b64 = auth::encode_nonce(&nonce);

    let challenge_envelope = Envelope::new(
        now_millis(),
        Message::AuthChallenge(AuthChallengePayload {
            ref_id: hello_id,
            nonce: nonce_b64,
        }),
    );
    let challenge_id = challenge_envelope.id;
    let json = encode(&challenge_envelope)?;
    ws_sink.send(tungstenite::Message::Text(json)).await?;

    tracing::info!(peer = %addr, "Sent auth.challenge");

    // Sign the nonce and send auth.proof.
    let signature_b64 = auth::sign_nonce(&inner.signing_key, &nonce);

    let proof_envelope = Envelope::new(
        now_millis(),
        Message::AuthProof(AuthProofPayload {
            ref_id: challenge_id,
            signature: signature_b64,
        }),
    );
    let json = encode(&proof_envelope)?;
    ws_sink.send(tungstenite::Message::Text(json)).await?;

    tracing::info!(peer = %addr, "Sent auth.proof");

    // --- Wait for auth.ok from agent ---
    let auth_deadline = tokio::time::Instant::now() + Duration::from_secs(10);
    loop {
        tokio::select! {
            _ = cancel.cancelled() => return Ok(()),
            _ = tokio::time::sleep_until(auth_deadline) => {
                tracing::warn!(peer = %addr, "Auth timeout — closing connection");
                return Err(CoreError::Auth("auth.ok timeout".into()));
            }
            msg = ws_stream_rx.next() => {
                match msg {
                    Some(Ok(tungstenite::Message::Text(text))) => {
                        let envelope = decode(&text)?;
                        match envelope.message {
                            Message::AuthOk(_) => {
                                tracing::info!(peer = %addr, "Authentication successful");
                                break;
                            }
                            Message::Error(err) => {
                                tracing::warn!(
                                    peer = %addr,
                                    code = ?err.code,
                                    "Agent rejected authentication"
                                );
                                return Err(CoreError::AuthFailed);
                            }
                            other => {
                                let got = format!("{:?}", std::mem::discriminant(&other));
                                return Err(CoreError::UnexpectedMessage {
                                    expected: "auth.ok".to_string(),
                                    got,
                                });
                            }
                        }
                    }
                    Some(Ok(tungstenite::Message::Close(_))) | None => {
                        return Err(CoreError::ConnectionClosed);
                    }
                    Some(Ok(_)) => continue,
                    Some(Err(e)) => return Err(CoreError::from(e)),
                }
            }
        }
    }

    // --- Session established ---
    let session = SessionInfo {
        id: Uuid::new_v4(),
        hostname: hello.hostname,
        username: hello.username,
        os: hello.os,
        os_version: hello.os_version,
        agent_version: hello.agent_version,
        capabilities: hello.capabilities,
        addr,
        connected: true,
        last_seen: Instant::now(),
    };

    let sid = session.id;
    registry.insert(session.clone()).await;
    let _ = event_tx.send(SessionEvent::Connected(session)).await;

    // --- Main loop: heartbeat sending + message receiving ---
    let mut heartbeat_interval = tokio::time::interval(HEARTBEAT_INTERVAL);
    heartbeat_interval.set_missed_tick_behavior(tokio::time::MissedTickBehavior::Delay);

    loop {
        tokio::select! {
            _ = cancel.cancelled() => {
                registry.mark_disconnected(&sid).await;
                let _ = event_tx.send(SessionEvent::Disconnected(sid)).await;
                return Ok(());
            }
            _ = heartbeat_interval.tick() => {
                let envelope = Envelope::new(
                    now_millis(),
                    Message::Heartbeat(HeartbeatPayload {}),
                );
                let json = encode(&envelope)?;
                if let Err(e) = ws_sink.send(tungstenite::Message::Text(json)).await {
                    tracing::warn!(session = %sid, error = %e, "Failed to send heartbeat");
                    registry.mark_disconnected(&sid).await;
                    let _ = event_tx.send(SessionEvent::Disconnected(sid)).await;
                    return Err(CoreError::from(e));
                }
            }
            msg = ws_stream_rx.next() => {
                match msg {
                    Some(Ok(tungstenite::Message::Text(text))) => {
                        let envelope = match decode(&text) {
                            Ok(e) => e,
                            Err(e) => {
                                tracing::warn!(session = %sid, error = %e, "Bad message");
                                continue;
                            }
                        };
                        tracing::trace!(
                            session = %sid,
                            msg_type = ?std::mem::discriminant(&envelope.message),
                            "Received message"
                        );
                        registry.update_last_seen(&sid).await;
                    }
                    Some(Ok(tungstenite::Message::Close(_))) | None => {
                        tracing::info!(session = %sid, "Agent disconnected");
                        registry.mark_disconnected(&sid).await;
                        let _ = event_tx.send(SessionEvent::Disconnected(sid)).await;
                        return Ok(());
                    }
                    Some(Ok(_)) => {
                        registry.update_last_seen(&sid).await;
                    }
                    Some(Err(e)) => {
                        tracing::warn!(session = %sid, error = %e, "WebSocket error");
                        registry.mark_disconnected(&sid).await;
                        let _ = event_tx.send(SessionEvent::Disconnected(sid)).await;
                        return Err(CoreError::from(e));
                    }
                }
            }
        }
    }
}

fn now_millis() -> u64 {
    std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .unwrap_or_default()
        .as_millis() as u64
}
