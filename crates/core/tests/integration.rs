//! Integration tests for discovery, server, and client.

use std::net::{Ipv4Addr, SocketAddr, SocketAddrV4};
use std::time::Duration;

use aula_core::client::{Client, ClientConfig};
use aula_core::discovery::{Announcement, Announcer, Listener};
use aula_core::server::{Server, ServerConfig, SessionEvent, SessionRegistry};
use aula_protocol::HelloPayload;
use tokio_util::sync::CancellationToken;

fn test_announcement(class_id: &str, ws_port: u16) -> Announcement {
    Announcement {
        aula: 1,
        class_id: class_id.to_string(),
        console_name: "Test Console".to_string(),
        ws_port,
        cert_fingerprint: "SHA256:0000000000000000000000000000000000000000000000000000000000000000"
            .to_string(),
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

/// Two clients connect to a server, appear in the registry, then one disconnects
/// and the registry marks it as disconnected after the heartbeat timeout.
#[tokio::test]
async fn server_two_clients_connect_and_disconnect() {
    let _ = tracing_subscriber::fmt().with_test_writer().try_init();

    let cancel = CancellationToken::new();
    let registry = SessionRegistry::new();

    // Bind a listener to find a free port, then let the server use it.
    let listener = tokio::net::TcpListener::bind("127.0.0.1:0").await.unwrap();
    let server_addr = listener.local_addr().unwrap();
    drop(listener); // free the port so the server can bind it

    let server = Server::new(
        ServerConfig {
            bind_addr: server_addr,
        },
        registry.clone(),
    );

    let mut events = server.run(cancel.clone()).await.unwrap();

    // Give server a moment to start listening.
    tokio::time::sleep(Duration::from_millis(100)).await;

    // Start two clients.
    let cancel_c1 = cancel.clone();
    let addr = server_addr;
    let client1_handle = tokio::spawn(async move {
        let client = Client::new(ClientConfig {
            hello: test_hello("PC-01"),
        });
        client.run(addr, cancel_c1).await
    });

    let cancel_c2 = cancel.clone();
    let client2_handle = tokio::spawn(async move {
        let client = Client::new(ClientConfig {
            hello: test_hello("PC-02"),
        });
        client.run(addr, cancel_c2).await
    });

    // Wait for both clients to connect.
    let mut connected_count = 0;
    let mut session_ids = Vec::new();
    let deadline = tokio::time::Instant::now() + Duration::from_secs(5);

    while connected_count < 2 {
        tokio::select! {
            _ = tokio::time::sleep_until(deadline) => {
                panic!("Timeout waiting for clients to connect");
            }
            event = events.recv() => {
                if let Some(SessionEvent::Connected(info)) = event {
                    connected_count += 1;
                    session_ids.push(info.id);
                }
            }
        }
    }

    assert_eq!(registry.connected_count().await, 2);

    // Verify hostnames are in the registry.
    let snapshot = registry.snapshot().await;
    let hostnames: Vec<&str> = snapshot.iter().map(|s| s.hostname.as_str()).collect();
    assert!(hostnames.contains(&"PC-01"));
    assert!(hostnames.contains(&"PC-02"));

    // Drop client1 by cancelling only it — we need individual cancellation tokens.
    // Instead, abort client1's task to simulate a crash.
    client1_handle.abort();

    // Wait for the heartbeat timeout to mark client1 as disconnected.
    let deadline = tokio::time::Instant::now() + Duration::from_secs(20);
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

    // One should be disconnected now.
    assert_eq!(registry.connected_count().await, 1);

    // Clean up.
    cancel.cancel();
    let _ = client2_handle.await;
}

/// An announcement with a different class_id is ignored by the listener.
#[tokio::test]
async fn listener_ignores_wrong_class_id() {
    let _ = tracing_subscriber::fmt().with_test_writer().try_init();

    let cancel = CancellationToken::new();

    // Use a non-standard port to avoid conflicts.
    let listener_port = {
        let sock = tokio::net::UdpSocket::bind("127.0.0.1:0").await.unwrap();
        sock.local_addr().unwrap().port()
    };

    // Start listener expecting "lab-A".
    let listener = Listener::new("lab-A".to_string()).with_port(listener_port);
    let mut rx = listener.run(cancel.clone()).await.unwrap();

    // Send an announcement with "lab-B" (wrong class_id).
    let wrong_announcement = test_announcement("lab-B", 47801);
    let payload = serde_json::to_vec(&wrong_announcement).unwrap();

    let sender = tokio::net::UdpSocket::bind("127.0.0.1:0").await.unwrap();
    sender
        .send_to(&payload, format!("127.0.0.1:{listener_port}"))
        .await
        .unwrap();

    // Send a correct announcement with "lab-A".
    let correct_announcement = test_announcement("lab-A", 47801);
    let payload = serde_json::to_vec(&correct_announcement).unwrap();
    sender
        .send_to(&payload, format!("127.0.0.1:{listener_port}"))
        .await
        .unwrap();

    // The listener should only yield the correct one.
    let discovered = tokio::time::timeout(Duration::from_secs(3), rx.recv())
        .await
        .expect("Timeout waiting for announcement")
        .expect("Channel closed");

    assert_eq!(discovered.announcement.class_id, "lab-A");

    // There should be no more pending messages (the wrong one was filtered).
    assert!(rx.try_recv().is_err());

    cancel.cancel();
}

/// Full loopback integration: announcer → listener → client connects to server.
///
/// The announcer broadcasts on all interfaces, so the listener may receive the
/// announcement from a non-loopback IP.  We bind the server on `0.0.0.0` so the
/// client can reach it regardless of which interface the announcement came from.
#[tokio::test]
async fn discovery_and_connection_loopback() {
    let _ = tracing_subscriber::fmt().with_test_writer().try_init();

    let cancel = CancellationToken::new();
    let registry = SessionRegistry::new();

    // Bind on all interfaces so the discovered IP can connect.
    let listener = tokio::net::TcpListener::bind("0.0.0.0:0").await.unwrap();
    let server_port = listener.local_addr().unwrap().port();
    drop(listener);

    let server = Server::new(
        ServerConfig {
            bind_addr: SocketAddr::V4(SocketAddrV4::new(Ipv4Addr::UNSPECIFIED, server_port)),
        },
        registry.clone(),
    );

    let mut events = server.run(cancel.clone()).await.unwrap();

    // Start announcer broadcasting the server's port.
    let discovery_port = {
        let sock = tokio::net::UdpSocket::bind("127.0.0.1:0").await.unwrap();
        sock.local_addr().unwrap().port()
    };

    let announcement = test_announcement("test-class", server_port);
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

    // Connect a client to the discovered console.
    let console_addr = SocketAddr::new(
        std::net::IpAddr::V4(discovered.addr),
        discovered.announcement.ws_port,
    );
    let client_cancel = cancel.clone();
    tokio::spawn(async move {
        let client = Client::new(ClientConfig {
            hello: test_hello("DISCOVERED-PC"),
        });
        let _ = client.run(console_addr, client_cancel).await;
    });

    // Wait for the client to appear in the registry.
    let deadline = tokio::time::Instant::now() + Duration::from_secs(10);
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
