//! WebSocket client for the student's agent.
//!
//! Connects to the console, sends `hello`, maintains heartbeat, and
//! reconnects with exponential backoff on disconnection.

use std::net::SocketAddr;
use std::time::Duration;

use aula_protocol::{decode, encode, Envelope, HeartbeatPayload, HelloPayload, Message};
use futures_util::{SinkExt, StreamExt};
use tokio_tungstenite::tungstenite;

use crate::error::CoreError;

/// Minimum backoff before reconnecting.
const BACKOFF_MIN: Duration = Duration::from_secs(1);
/// Maximum backoff before reconnecting.
const BACKOFF_MAX: Duration = Duration::from_secs(30);
/// Heartbeat send interval.
const HEARTBEAT_INTERVAL: Duration = Duration::from_secs(5);
/// If no message arrives within this window, the connection is considered dead.
const HEARTBEAT_TIMEOUT: Duration = Duration::from_secs(15);

/// Configuration for the agent client.
#[derive(Debug, Clone)]
pub struct ClientConfig {
    /// The hello payload to send on connection.
    pub hello: HelloPayload,
}

/// The agent-side WebSocket client.
pub struct Client {
    config: ClientConfig,
}

impl Client {
    pub fn new(config: ClientConfig) -> Self {
        Self { config }
    }

    /// Connect to the console at the given address and run until cancelled.
    ///
    /// Sends `hello`, maintains heartbeat every 5 s, and reconnects with
    /// exponential backoff if the connection drops.
    pub async fn run(
        &self,
        console_addr: SocketAddr,
        cancel: tokio_util::sync::CancellationToken,
    ) -> Result<(), CoreError> {
        let mut backoff = BACKOFF_MIN;

        loop {
            if cancel.is_cancelled() {
                return Ok(());
            }

            let url = format!("ws://{console_addr}"); // TODO(05): use wss:// with TLS
            tracing::info!(addr = %console_addr, "Connecting to console");

            match tokio_tungstenite::connect_async(&url).await {
                Ok((ws_stream, _)) => {
                    tracing::info!(addr = %console_addr, "Connected to console");
                    backoff = BACKOFF_MIN; // reset on successful connection

                    match self.run_session(ws_stream, &cancel).await {
                        Ok(()) => {
                            // Graceful shutdown (cancel triggered).
                            return Ok(());
                        }
                        Err(e) => {
                            tracing::warn!(error = %e, "Session ended");
                        }
                    }
                }
                Err(e) => {
                    tracing::warn!(
                        addr = %console_addr,
                        error = %e,
                        backoff_secs = backoff.as_secs(),
                        "Connection failed, will retry"
                    );
                }
            }

            // Wait with backoff before retrying.
            tokio::select! {
                _ = cancel.cancelled() => return Ok(()),
                _ = tokio::time::sleep(backoff) => {}
            }

            backoff = (backoff * 2).min(BACKOFF_MAX);
        }
    }

    /// Run a single session: hello → heartbeat loop.
    async fn run_session(
        &self,
        ws_stream: tokio_tungstenite::WebSocketStream<
            tokio_tungstenite::MaybeTlsStream<tokio::net::TcpStream>,
        >,
        cancel: &tokio_util::sync::CancellationToken,
    ) -> Result<(), CoreError> {
        let (mut ws_sink, mut ws_rx) = ws_stream.split();

        // Send hello.
        let hello_envelope = Envelope::new(now_millis(), Message::Hello(self.config.hello.clone()));
        let json = encode(&hello_envelope)?;
        ws_sink.send(tungstenite::Message::Text(json)).await?;

        tracing::info!(msg_type = "hello", "Sent hello");

        // TODO(05): wait for auth.challenge, send auth.proof, wait for auth.ok

        // Main loop: send heartbeats and receive messages.
        let mut heartbeat_interval = tokio::time::interval(HEARTBEAT_INTERVAL);
        heartbeat_interval.set_missed_tick_behavior(tokio::time::MissedTickBehavior::Delay);

        let timeout = tokio::time::sleep(HEARTBEAT_TIMEOUT);
        tokio::pin!(timeout);

        loop {
            tokio::select! {
                _ = cancel.cancelled() => {
                    let _ = ws_sink.send(tungstenite::Message::Close(None)).await;
                    return Ok(());
                }
                _ = heartbeat_interval.tick() => {
                    let envelope = Envelope::new(
                        now_millis(),
                        Message::Heartbeat(HeartbeatPayload {}),
                    );
                    let json = encode(&envelope)?;
                    ws_sink.send(tungstenite::Message::Text(json)).await?;
                    tracing::trace!("Heartbeat sent");
                }
                _ = &mut timeout => {
                    tracing::warn!("Heartbeat timeout — connection assumed dead");
                    return Err(CoreError::HeartbeatTimeout);
                }
                msg = ws_rx.next() => {
                    match msg {
                        Some(Ok(tungstenite::Message::Text(text))) => {
                            let envelope = match decode(&text) {
                                Ok(e) => e,
                                Err(e) => {
                                    tracing::warn!(error = %e, "Bad message from console");
                                    continue;
                                }
                            };
                            tracing::trace!(
                                msg_type = ?std::mem::discriminant(&envelope.message),
                                "Received message"
                            );
                            // Reset heartbeat timeout on any valid message.
                            timeout.as_mut().reset(tokio::time::Instant::now() + HEARTBEAT_TIMEOUT);
                        }
                        Some(Ok(tungstenite::Message::Close(_))) | None => {
                            tracing::info!("Console closed connection");
                            return Err(CoreError::ConnectionClosed);
                        }
                        Some(Ok(_)) => {
                            // ping/pong/binary — reset timeout
                            timeout.as_mut().reset(tokio::time::Instant::now() + HEARTBEAT_TIMEOUT);
                        }
                        Some(Err(e)) => {
                            return Err(CoreError::from(e));
                        }
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
