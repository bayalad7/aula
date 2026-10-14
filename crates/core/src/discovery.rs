//! UDP broadcast discovery for console announcements and agent listening.

use std::net::{Ipv4Addr, SocketAddr, SocketAddrV4};
use std::time::Duration;

use serde::{Deserialize, Serialize};
use tokio::net::UdpSocket;
use tokio::sync::mpsc;

use crate::error::CoreError;

/// Default UDP discovery port.
pub const DISCOVERY_PORT: u16 = 47800;

/// Default announcement interval.
const ANNOUNCE_INTERVAL: Duration = Duration::from_secs(2);

/// UDP broadcast announcement sent by the console.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct Announcement {
    /// Protocol version for discovery (always 1).
    pub aula: u32,
    /// Class/group identifier.
    pub class_id: String,
    /// Human-readable console name.
    pub console_name: String,
    /// TCP port where the console accepts WebSocket connections.
    pub ws_port: u16,
    /// SHA-256 fingerprint of the console's TLS certificate.
    pub cert_fingerprint: String,
}

/// Information about a discovered console.
#[derive(Debug, Clone)]
pub struct DiscoveredConsole {
    /// The IP address the announcement came from.
    pub addr: Ipv4Addr,
    /// The announcement data.
    pub announcement: Announcement,
}

/// Broadcasts console announcements on all non-loopback IPv4 interfaces.
pub struct Announcer {
    announcement: Announcement,
    port: u16,
}

impl Announcer {
    pub fn new(announcement: Announcement) -> Self {
        Self {
            announcement,
            port: DISCOVERY_PORT,
        }
    }

    /// Override the target port (useful for testing).
    pub fn with_port(mut self, port: u16) -> Self {
        self.port = port;
        self
    }

    /// Run the announcer until the cancellation token is triggered.
    ///
    /// Sends the announcement as a UDP broadcast every 2 seconds on each
    /// non-loopback IPv4 interface.
    pub async fn run(&self, cancel: tokio_util::sync::CancellationToken) -> Result<(), CoreError> {
        let payload = serde_json::to_vec(&self.announcement)?;
        let broadcast_addr = SocketAddrV4::new(Ipv4Addr::BROADCAST, self.port);

        let socket = UdpSocket::bind(SocketAddrV4::new(Ipv4Addr::UNSPECIFIED, 0)).await?;
        socket.set_broadcast(true)?;

        tracing::info!(
            class_id = %self.announcement.class_id,
            port = self.port,
            "Announcer started"
        );

        loop {
            tokio::select! {
                _ = cancel.cancelled() => {
                    tracing::info!("Announcer stopped");
                    return Ok(());
                }
                _ = tokio::time::sleep(ANNOUNCE_INTERVAL) => {
                    if let Err(e) = socket.send_to(&payload, broadcast_addr).await {
                        tracing::warn!(error = %e, "Failed to send announcement");
                    } else {
                        tracing::trace!("Announcement sent");
                    }
                }
            }
        }
    }
}

/// Listens for console announcements, filtering by `class_id`.
pub struct Listener {
    expected_class_id: String,
    port: u16,
}

impl Listener {
    pub fn new(expected_class_id: String) -> Self {
        Self {
            expected_class_id,
            port: DISCOVERY_PORT,
        }
    }

    /// Override the listen port (useful for testing).
    pub fn with_port(mut self, port: u16) -> Self {
        self.port = port;
        self
    }

    /// Run the listener, sending discovered consoles to the returned channel.
    ///
    /// Only announcements matching the expected `class_id` are forwarded.
    /// The `cert_fingerprint` check is skipped for now.
    pub async fn run(
        &self,
        cancel: tokio_util::sync::CancellationToken,
    ) -> Result<mpsc::Receiver<DiscoveredConsole>, CoreError> {
        let (tx, rx) = mpsc::channel(16);

        let socket = UdpSocket::bind(SocketAddrV4::new(Ipv4Addr::UNSPECIFIED, self.port)).await?;
        socket.set_broadcast(true)?;

        let expected_class_id = self.expected_class_id.clone();

        tracing::info!(
            class_id = %expected_class_id,
            port = self.port,
            "Listener started"
        );

        tokio::spawn(async move {
            let mut buf = [0u8; 4096];
            loop {
                tokio::select! {
                    _ = cancel.cancelled() => {
                        tracing::info!("Listener stopped");
                        return;
                    }
                    result = socket.recv_from(&mut buf) => {
                        match result {
                            Ok((len, src)) => {
                                Self::handle_datagram(
                                    &buf[..len],
                                    src,
                                    &expected_class_id,
                                    &tx,
                                ).await;
                            }
                            Err(e) => {
                                tracing::warn!(error = %e, "Error receiving datagram");
                            }
                        }
                    }
                }
            }
        });

        Ok(rx)
    }

    async fn handle_datagram(
        data: &[u8],
        src: SocketAddr,
        expected_class_id: &str,
        tx: &mpsc::Sender<DiscoveredConsole>,
    ) {
        let announcement: Announcement = match serde_json::from_slice(data) {
            Ok(a) => a,
            Err(e) => {
                tracing::debug!(error = %e, "Ignoring malformed announcement");
                return;
            }
        };

        if announcement.aula != 1 {
            tracing::debug!(
                version = announcement.aula,
                "Ignoring unknown discovery version"
            );
            return;
        }

        if announcement.class_id != expected_class_id {
            tracing::debug!(
                received = %announcement.class_id,
                expected = %expected_class_id,
                "Ignoring announcement with different class_id"
            );
            return;
        }

        // TODO(05): verify cert_fingerprint against configured public key

        let addr = match src {
            SocketAddr::V4(v4) => *v4.ip(),
            SocketAddr::V6(_) => {
                tracing::debug!("Ignoring IPv6 source");
                return;
            }
        };

        let console = DiscoveredConsole { addr, announcement };

        tracing::info!(addr = %addr, "Console discovered");

        if tx.send(console).await.is_err() {
            tracing::debug!("Listener channel closed");
        }
    }
}
