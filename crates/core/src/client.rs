//! WebSocket client for the student's agent.
//!
//! Connects to the console over TLS with certificate pinning, verifies
//! Ed25519 authentication, sends `hello`, maintains heartbeat, and
//! reconnects with exponential backoff on disconnection.

use std::net::SocketAddr;
use std::time::Duration;

use aula_protocol::{
    decode, encode, AuthOkPayload, Envelope, ErrorCode, ErrorPayload, HeartbeatPayload,
    HelloPayload, Message,
};
use ed25519_dalek::VerifyingKey;
use futures_util::{SinkExt, StreamExt};
use rustls::pki_types::ServerName;
use tokio::net::TcpStream;
use tokio_tungstenite::tungstenite;

use crate::auth;
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
#[derive(Clone)]
pub struct ClientConfig {
    /// The hello payload to send on connection.
    pub hello: HelloPayload,
    /// The console's Ed25519 public key (for verifying auth.proof).
    pub console_public_key: VerifyingKey,
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
    /// `expected_fingerprint` is the `SHA256:<hex>` fingerprint from the UDP
    /// announcement, used for TLS certificate pinning.
    ///
    /// Sends `hello`, performs Ed25519 authentication, maintains heartbeat
    /// every 5 s, and reconnects with exponential backoff if the connection
    /// drops.
    pub async fn run(
        &self,
        console_addr: SocketAddr,
        expected_fingerprint: &str,
        cancel: tokio_util::sync::CancellationToken,
    ) -> Result<(), CoreError> {
        let tls_config = crate::tls::client_config_pinned(expected_fingerprint)?;
        let tls_connector = tokio_rustls::TlsConnector::from(tls_config);

        let mut backoff = BACKOFF_MIN;

        loop {
            if cancel.is_cancelled() {
                return Ok(());
            }

            tracing::info!(addr = %console_addr, "Connecting to console (TLS)");

            match self
                .try_connect(console_addr, &tls_connector, &cancel)
                .await
            {
                Ok(ws_stream) => {
                    tracing::info!(addr = %console_addr, "Connected to console (TLS)");
                    backoff = BACKOFF_MIN;

                    match self.run_session(ws_stream, &cancel).await {
                        Ok(()) => return Ok(()),
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

            tokio::select! {
                _ = cancel.cancelled() => return Ok(()),
                _ = tokio::time::sleep(backoff) => {}
            }

            backoff = (backoff * 2).min(BACKOFF_MAX);
        }
    }

    /// Establish TCP -> TLS -> WebSocket connection.
    async fn try_connect(
        &self,
        addr: SocketAddr,
        tls_connector: &tokio_rustls::TlsConnector,
        cancel: &tokio_util::sync::CancellationToken,
    ) -> Result<
        tokio_tungstenite::WebSocketStream<tokio_rustls::client::TlsStream<TcpStream>>,
        CoreError,
    > {
        // TCP connect.
        let tcp_stream = tokio::select! {
            _ = cancel.cancelled() => return Err(CoreError::ConnectionClosed),
            result = TcpStream::connect(addr) => result?,
        };

        // TLS handshake with certificate pinning.
        // Use a dummy server name — the pinning verifier ignores it.
        let server_name = ServerName::try_from("aula.local")
            .map_err(|e| CoreError::Tls(format!("invalid server name: {e}")))?;
        let tls_stream = tls_connector
            .connect(server_name, tcp_stream)
            .await
            .map_err(|e| CoreError::Tls(format!("TLS handshake failed: {e}")))?;

        // WebSocket handshake over TLS.
        let url = format!("wss://{addr}");
        let (ws_stream, _) = tokio_tungstenite::client_async(&url, tls_stream)
            .await
            .map_err(CoreError::from)?;

        Ok(ws_stream)
    }

    /// Run a single session: hello -> auth -> heartbeat loop.
    async fn run_session(
        &self,
        ws_stream: tokio_tungstenite::WebSocketStream<tokio_rustls::client::TlsStream<TcpStream>>,
        cancel: &tokio_util::sync::CancellationToken,
    ) -> Result<(), CoreError> {
        let (mut ws_sink, mut ws_rx) = ws_stream.split();

        // --- Send hello ---
        let hello_envelope = Envelope::new(now_millis(), Message::Hello(self.config.hello.clone()));
        let json = encode(&hello_envelope)?;
        ws_sink.send(tungstenite::Message::Text(json)).await?;

        tracing::info!(msg_type = "hello", "Sent hello");

        // --- Wait for auth.challenge ---
        let nonce_b64 = loop {
            tokio::select! {
                _ = cancel.cancelled() => return Ok(()),
                msg = ws_rx.next() => {
                    match msg {
                        Some(Ok(tungstenite::Message::Text(text))) => {
                            let envelope = decode(&text)?;
                            match envelope.message {
                                Message::AuthChallenge(challenge) => {
                                    tracing::info!("Received auth.challenge");
                                    break challenge.nonce;
                                }
                                other => {
                                    let got = format!("{:?}", std::mem::discriminant(&other));
                                    return Err(CoreError::UnexpectedMessage {
                                        expected: "auth.challenge".to_string(),
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

        // --- Wait for auth.proof ---
        let (proof_id, signature_b64) = loop {
            tokio::select! {
                _ = cancel.cancelled() => return Ok(()),
                msg = ws_rx.next() => {
                    match msg {
                        Some(Ok(tungstenite::Message::Text(text))) => {
                            let envelope = decode(&text)?;
                            match envelope.message {
                                Message::AuthProof(proof) => {
                                    tracing::info!("Received auth.proof");
                                    break (envelope.id, proof.signature);
                                }
                                other => {
                                    let got = format!("{:?}", std::mem::discriminant(&other));
                                    return Err(CoreError::UnexpectedMessage {
                                        expected: "auth.proof".to_string(),
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

        // --- Verify signature ---
        let nonce_bytes = auth::decode_nonce(&nonce_b64)?;
        match auth::verify_signature(
            &self.config.console_public_key,
            &nonce_bytes,
            &signature_b64,
        ) {
            Ok(()) => {
                tracing::info!("Ed25519 signature verified — console authenticated");
                // Send auth.ok.
                let ok_envelope = Envelope::new(
                    now_millis(),
                    Message::AuthOk(AuthOkPayload { ref_id: proof_id }),
                );
                let json = encode(&ok_envelope)?;
                ws_sink.send(tungstenite::Message::Text(json)).await?;
            }
            Err(_) => {
                tracing::warn!("Ed25519 signature verification FAILED — rejecting console");
                // Send error and close.
                let err_envelope = Envelope::new(
                    now_millis(),
                    Message::Error(ErrorPayload {
                        ref_id: proof_id,
                        code: ErrorCode::AuthFailed,
                        message: "signature verification failed".into(),
                    }),
                );
                let json = encode(&err_envelope)?;
                let _ = ws_sink.send(tungstenite::Message::Text(json)).await;
                let _ = ws_sink.send(tungstenite::Message::Close(None)).await;
                return Err(CoreError::AuthFailed);
            }
        }

        // --- Main loop: send heartbeats and receive messages ---
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
                            timeout.as_mut().reset(tokio::time::Instant::now() + HEARTBEAT_TIMEOUT);
                        }
                        Some(Ok(tungstenite::Message::Close(_))) | None => {
                            tracing::info!("Console closed connection");
                            return Err(CoreError::ConnectionClosed);
                        }
                        Some(Ok(_)) => {
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
