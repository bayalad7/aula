//! Integration tests for discovery, TLS, authentication, server, and client.

use std::net::{Ipv4Addr, SocketAddr, SocketAddrV4};
use std::time::Duration;

use aula_core::auth;
use aula_core::client::{Client, ClientConfig};
use aula_core::discovery::{Announcement, Announcer, Listener};
use aula_core::server::{Server, ServerConfig, SessionEvent, SessionRegistry};
use aula_core::tls;
use aula_protocol::HelloPayload;
use tokio_util::sync::CancellationToken;

/// Helper: generate a matching TLS identity + Ed25519 keypair for tests.
fn test_crypto() -> (tls::TlsIdentity, ed25519_dalek::SigningKey) {
    let identity = tls::generate_self_signed("Test Console").unwrap();
    let signing_key = auth::generate_keypair();
    (identity, signing_key)
}

fn test_announcement(class_id: &str, ws_port: u16, fingerprint: &str) -> Announcement {
    Announcement {
        aula: 1,
        class_id: class_id.to_string(),
        console_name: "Test Console".to_string(),
        ws_port,
        cert_fingerprint: fingerprint.to_string(),
    }
}

fn test_hello(hostname: &str) -> HelloPayload {
    HelloPayload {
        hostname: hostname.to_string(),
        username: "test-user".to_string(),
        os: "windows".to_string(),
        os_version: "10.0".to_string(),
        agent_version: "0.1.0".to_string(),
        capabilities: vec![],
    }
}

/// Helper: start a TLS server with the given crypto material.
async fn start_server(
    identity: tls::TlsIdentity,
    signing_key: ed25519_dalek::SigningKey,
    cancel: CancellationToken,
) -> (
    SocketAddr,
    SessionRegistry,
    tokio::sync::mpsc::Receiver<SessionEvent>,
) {
    let listener = tokio::net::TcpListener::bind("127.0.0.1:0").await.unwrap();
    let addr = listener.local_addr().unwrap();
    drop(listener);

    let registry = SessionRegistry::new();
    let server = Server::new(
        ServerConfig {
            bind_addr: addr,
            tls_identity: identity,
            signing_key,
        },
        registry.clone(),
    )
    .unwrap();

    let events = server.run(cancel).await.unwrap();
    // Give server a moment to start.
    tokio::time::sleep(Duration::from_millis(100)).await;

    (addr, registry, events)
}

/// Two clients connect to a TLS server with correct Ed25519 keys,
/// authenticate successfully, appear in the registry, then one disconnects.
#[tokio::test]
async fn server_two_clients_tls_auth_connect_and_disconnect() {
    let _ = tracing_subscriber::fmt().with_test_writer().try_init();

    let (identity, signing_key) = test_crypto();
    let fingerprint = identity.fingerprint.clone();
    let public_key = signing_key.verifying_key();

    let cancel = CancellationToken::new();
    let (server_addr, registry, mut events) =
        start_server(identity, signing_key, cancel.clone()).await;

    // Start two clients with matching keys.
    let cancel_c1 = cancel.clone();
    let fp = fingerprint.clone();
    let pk = public_key;
    let client1_handle = tokio::spawn(async move {
        let client = Client::new(ClientConfig {
            hello: test_hello("PC-01"),
            console_public_key: pk,
        });
        client.run(server_addr, &fp, cancel_c1).await
    });

    let cancel_c2 = cancel.clone();
    let fp = fingerprint.clone();
    let pk = public_key;
    let _client2_handle = tokio::spawn(async move {
        let client = Client::new(ClientConfig {
            hello: test_hello("PC-02"),
            console_public_key: pk,
        });
        client.run(server_addr, &fp, cancel_c2).await
    });

    // Wait for both clients to connect.
    let mut connected_count = 0;
    let deadline = tokio::time::Instant::now() + Duration::from_secs(10);

    while connected_count < 2 {
        tokio::select! {
            _ = tokio::time::sleep_until(deadline) => {
                panic!("Timeout waiting for clients to connect");
            }
            event = events.recv() => {
                if let Some(SessionEvent::Connected(info)) = event {
                    connected_count += 1;
                    assert!(info.hostname == "PC-01" || info.hostname == "PC-02");
                }
            }
        }
    }

    assert_eq!(registry.connected_count().await, 2);

    let snapshot = registry.snapshot().await;
    let hostnames: Vec<&str> = snapshot.iter().map(|s| s.hostname.as_str()).collect();
    assert!(hostnames.contains(&"PC-01"));
    assert!(hostnames.contains(&"PC-02"));

    // Abort client1 to simulate crash.
    client1_handle.abort();

    // Wait for heartbeat timeout to detect disconnection.
    let deadline = tokio::time::Instant::now() + Duration::from_secs(25);
    loop {
        tokio::select! {
            _ = tokio::time::sleep_until(deadline) => {
                panic!("Timeout waiting for disconnect detection");
            }
            event = events.recv() => {
                if let Some(SessionEvent::Disconnected(_)) = event {
                    break;
                }
            }
        }
    }

    assert_eq!(registry.connected_count().await, 1);
    cancel.cancel();
}

/// An agent with the WRONG Ed25519 public key rejects the console.
/// The server should detect that the agent rejected authentication.
#[tokio::test]
async fn auth_wrong_key_rejected() {
    let _ = tracing_subscriber::fmt().with_test_writer().try_init();

    let (identity, signing_key) = test_crypto();
    let fingerprint = identity.fingerprint.clone();

    // Generate a DIFFERENT keypair — the client will use this wrong public key.
    let wrong_key = auth::generate_keypair();
    let wrong_public_key = wrong_key.verifying_key();

    let cancel = CancellationToken::new();
    let (server_addr, registry, _events) =
        start_server(identity, signing_key, cancel.clone()).await;

    // Client uses the wrong public key.
    let client_cancel = CancellationToken::new();
    let cc = client_cancel.clone();
    let fp = fingerprint.clone();
    let handle = tokio::spawn(async move {
        let client = Client::new(ClientConfig {
            hello: test_hello("WRONG-KEY-PC"),
            console_public_key: wrong_public_key,
        });
        client.run(server_addr, &fp, cc).await
    });

    // The client should fail auth — give it time.
    tokio::time::sleep(Duration::from_secs(3)).await;

    // No session should appear in the registry.
    assert_eq!(registry.connected_count().await, 0);

    // Clean up.
    client_cancel.cancel();
    cancel.cancel();
    let _ = handle.await;
}

/// A client connecting with a WRONG TLS fingerprint should be rejected
/// before the WebSocket handshake even begins.
#[tokio::test]
async fn tls_wrong_fingerprint_rejected() {
    let _ = tracing_subscriber::fmt().with_test_writer().try_init();

    let (identity, signing_key) = test_crypto();
    let _real_fingerprint = identity.fingerprint.clone();

    let cancel = CancellationToken::new();
    let (server_addr, registry, _events) =
        start_server(identity, signing_key, cancel.clone()).await;

    // Client uses a bogus fingerprint.
    let wrong_fp = "SHA256:0000000000000000000000000000000000000000000000000000000000000000";

    let client_cancel = CancellationToken::new();
    let cc = client_cancel.clone();
    let pk = auth::generate_keypair().verifying_key();
    let handle = tokio::spawn(async move {
        let client = Client::new(ClientConfig {
            hello: test_hello("WRONG-FP-PC"),
            console_public_key: pk,
        });
        client.run(server_addr, wrong_fp, cc).await
    });

    // Give the client time to attempt and fail.
    tokio::time::sleep(Duration::from_secs(3)).await;

    // No session should appear.
    assert_eq!(registry.connected_count().await, 0);

    client_cancel.cancel();
    cancel.cancel();
    let _ = handle.await;
}

/// An announcement with a different class_id is ignored by the listener.
#[tokio::test]
async fn listener_ignores_wrong_class_id() {
    let _ = tracing_subscriber::fmt().with_test_writer().try_init();

    let cancel = CancellationToken::new();

    let listener_port = {
        let sock = tokio::net::UdpSocket::bind("127.0.0.1:0").await.unwrap();
        sock.local_addr().unwrap().port()
    };

    let listener = Listener::new("lab-A".to_string()).with_port(listener_port);
    let mut rx = listener.run(cancel.clone()).await.unwrap();

    let wrong_announcement = test_announcement(
        "lab-B",
        47801,
        "SHA256:0000000000000000000000000000000000000000000000000000000000000000",
    );
    let payload = serde_json::to_vec(&wrong_announcement).unwrap();

    let sender = tokio::net::UdpSocket::bind("127.0.0.1:0").await.unwrap();
    sender
        .send_to(&payload, format!("127.0.0.1:{listener_port}"))
        .await
        .unwrap();

    let correct_announcement = test_announcement(
        "lab-A",
        47801,
        "SHA256:0000000000000000000000000000000000000000000000000000000000000000",
    );
    let payload = serde_json::to_vec(&correct_announcement).unwrap();
    sender
        .send_to(&payload, format!("127.0.0.1:{listener_port}"))
        .await
        .unwrap();

    let discovered = tokio::time::timeout(Duration::from_secs(3), rx.recv())
        .await
        .expect("Timeout waiting for announcement")
        .expect("Channel closed");

    assert_eq!(discovered.announcement.class_id, "lab-A");
    assert!(rx.try_recv().is_err());

    cancel.cancel();
}

/// Full loopback integration: announcer -> listener -> TLS client connects
/// to TLS server with Ed25519 authentication.
#[tokio::test]
async fn discovery_and_tls_connection_loopback() {
    let _ = tracing_subscriber::fmt().with_test_writer().try_init();

    let (identity, signing_key) = test_crypto();
    let fingerprint = identity.fingerprint.clone();
    let public_key = signing_key.verifying_key();

    let cancel = CancellationToken::new();

    // Bind on all interfaces.
    let tcp_listener = tokio::net::TcpListener::bind("0.0.0.0:0").await.unwrap();
    let server_port = tcp_listener.local_addr().unwrap().port();
    drop(tcp_listener);

    let registry = SessionRegistry::new();
    let server = Server::new(
        ServerConfig {
            bind_addr: SocketAddr::V4(SocketAddrV4::new(Ipv4Addr::UNSPECIFIED, server_port)),
            tls_identity: identity,
            signing_key,
        },
        registry.clone(),
    )
    .unwrap();

    let mut events = server.run(cancel.clone()).await.unwrap();

    // Start announcer.
    let discovery_port = {
        let sock = tokio::net::UdpSocket::bind("127.0.0.1:0").await.unwrap();
        sock.local_addr().unwrap().port()
    };

    let announcement = test_announcement("test-class", server_port, &fingerprint);
    let announcer = Announcer::new(announcement).with_port(discovery_port);
    let announcer_cancel = cancel.clone();
    tokio::spawn(async move {
        let _ = announcer.run(announcer_cancel).await;
    });

    // Start listener.
    let disc_listener = Listener::new("test-class".to_string()).with_port(discovery_port);
    let mut disc_rx = disc_listener.run(cancel.clone()).await.unwrap();

    // Wait for discovery.
    let discovered = tokio::time::timeout(Duration::from_secs(10), disc_rx.recv())
        .await
        .expect("Timeout waiting for discovery")
        .expect("Channel closed");

    assert_eq!(discovered.announcement.ws_port, server_port);

    // Connect a TLS client to the discovered console.
    let console_addr = SocketAddr::new(
        std::net::IpAddr::V4(discovered.addr),
        discovered.announcement.ws_port,
    );
    let disc_fingerprint = discovered.announcement.cert_fingerprint.clone();
    let client_cancel = cancel.clone();
    let pk = public_key;
    tokio::spawn(async move {
        let client = Client::new(ClientConfig {
            hello: test_hello("DISCOVERED-PC"),
            console_public_key: pk,
        });
        let _ = client
            .run(console_addr, &disc_fingerprint, client_cancel)
            .await;
    });

    // Wait for the client to appear in the registry.
    let deadline = tokio::time::Instant::now() + Duration::from_secs(15);
    loop {
        tokio::select! {
            _ = tokio::time::sleep_until(deadline) => {
                panic!("Timeout waiting for client to connect via discovery");
            }
            event = events.recv() => {
                if let Some(SessionEvent::Connected(info)) = event {
                    assert_eq!(info.hostname, "DISCOVERED-PC");
                    break;
                }
            }
        }
    }

    assert_eq!(registry.connected_count().await, 1);
    cancel.cancel();
}

/// Verify that private key material never appears in tracing output.
#[tokio::test]
async fn private_key_not_in_logs() {
    use std::sync::{Arc, Mutex};
    use tracing_subscriber::fmt::MakeWriter;

    /// A writer that captures all log output into a shared buffer.
    #[derive(Clone)]
    struct BufWriter(Arc<Mutex<Vec<u8>>>);

    impl std::io::Write for BufWriter {
        fn write(&mut self, buf: &[u8]) -> std::io::Result<usize> {
            self.0.lock().unwrap().extend_from_slice(buf);
            Ok(buf.len())
        }
        fn flush(&mut self) -> std::io::Result<()> {
            Ok(())
        }
    }

    impl<'a> MakeWriter<'a> for BufWriter {
        type Writer = BufWriter;
        fn make_writer(&'a self) -> Self::Writer {
            self.clone()
        }
    }

    let buf = BufWriter(Arc::new(Mutex::new(Vec::new())));
    let subscriber = tracing_subscriber::fmt()
        .with_writer(buf.clone())
        .with_max_level(tracing::Level::TRACE)
        .finish();

    let _guard = tracing::subscriber::set_default(subscriber);

    // Generate keys and run a short connection.
    let (identity, signing_key) = test_crypto();
    let fingerprint = identity.fingerprint.clone();
    let public_key = signing_key.verifying_key();

    // Capture the private key bytes in base64 to search for them.
    let priv_key_b64 = base64::Engine::encode(
        &base64::engine::general_purpose::STANDARD,
        signing_key.to_bytes(),
    );

    let cancel = CancellationToken::new();
    let (server_addr, _registry, mut events) =
        start_server(identity, signing_key, cancel.clone()).await;

    let client_cancel = cancel.clone();
    let fp = fingerprint.clone();
    let pk = public_key;
    tokio::spawn(async move {
        let client = Client::new(ClientConfig {
            hello: test_hello("LOG-CHECK-PC"),
            console_public_key: pk,
        });
        let _ = client.run(server_addr, &fp, client_cancel).await;
    });

    // Wait for connection.
    let deadline = tokio::time::Instant::now() + Duration::from_secs(10);
    loop {
        tokio::select! {
            _ = tokio::time::sleep_until(deadline) => {
                panic!("Timeout waiting for connection");
            }
            event = events.recv() => {
                if let Some(SessionEvent::Connected(_)) = event {
                    break;
                }
            }
        }
    }

    cancel.cancel();
    tokio::time::sleep(Duration::from_millis(500)).await;

    // Check that private key bytes don't appear in log output.
    let binding = buf.0.lock().unwrap();
    let log_output = String::from_utf8_lossy(&binding);
    assert!(
        !log_output.contains(&priv_key_b64),
        "Private key (base64) found in log output!"
    );
}
