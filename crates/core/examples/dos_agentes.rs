//! Example: two simulated agents connect to a console server over TLS with
//! Ed25519 authentication, and then one disconnects.
//!
//! Run with:
//!   cargo run --example dos_agentes
//!
//! Expected output:
//! - Two agents appear in the session registry.
//! - After a few seconds, one agent is stopped.
//! - The registry detects the disconnection via heartbeat timeout.

use std::time::Duration;

use aula_core::auth;
use aula_core::client::{Client, ClientConfig};
use aula_core::server::{Server, ServerConfig, SessionEvent, SessionRegistry};
use aula_core::tls;
use aula_protocol::HelloPayload;
use tokio_util::sync::CancellationToken;

#[tokio::main]
async fn main() -> anyhow::Result<()> {
    tracing_subscriber::fmt()
        .with_max_level(tracing::Level::INFO)
        .init();

    // Generate TLS identity and Ed25519 keypair for the console.
    let tls_identity = tls::generate_self_signed("Test Console")?;
    let fingerprint = tls_identity.fingerprint.clone();
    let signing_key = auth::generate_keypair();
    let public_key = signing_key.verifying_key();

    let cancel = CancellationToken::new();
    let registry = SessionRegistry::new();

    // Start server on localhost with a random port.
    let listener = tokio::net::TcpListener::bind("127.0.0.1:0").await?;
    let server_addr = listener.local_addr()?;
    drop(listener);

    let server = Server::new(
        ServerConfig {
            bind_addr: server_addr,
            tls_identity,
            signing_key,
        },
        registry.clone(),
    )?;

    let mut events = server.run(cancel.clone()).await?;
    println!("Servidor escuchando en {server_addr} (TLS + Ed25519)");

    // Start two agents.
    let cancel_a1 = CancellationToken::new();
    let cancel_a1_child = cancel_a1.clone();
    let addr = server_addr;
    let fp = fingerprint.clone();
    let pk = public_key;
    tokio::spawn(async move {
        let client = Client::new(ClientConfig {
            hello: HelloPayload {
                hostname: "PC-LAB-01".to_string(),
                username: "alumno1".to_string(),
                os: "windows".to_string(),
                os_version: "10.0.19045".to_string(),
                agent_version: "0.1.0".to_string(),
                capabilities: vec![],
            },
            console_public_key: pk,
        });
        let _ = client.run(addr, &fp, cancel_a1_child).await;
    });

    let cancel_a2 = cancel.clone();
    let fp = fingerprint.clone();
    let pk = public_key;
    tokio::spawn(async move {
        let client = Client::new(ClientConfig {
            hello: HelloPayload {
                hostname: "PC-LAB-02".to_string(),
                username: "alumno2".to_string(),
                os: "linux".to_string(),
                os_version: "6.1.0".to_string(),
                agent_version: "0.1.0".to_string(),
                capabilities: vec![],
            },
            console_public_key: pk,
        });
        let _ = client.run(addr, &fp, cancel_a2).await;
    });

    // Wait for both agents to connect.
    let mut connected = 0;
    while connected < 2 {
        if let Some(SessionEvent::Connected(info)) = events.recv().await {
            connected += 1;
            println!(
                "Agente conectado: {} (usuario: {}, SO: {}) [{}/2]",
                info.hostname, info.username, info.os, connected
            );
        }
    }

    println!("\nRegistro actual:");
    for s in registry.snapshot().await {
        println!(
            "  - {} | {} | {} | conectado: {}",
            s.hostname, s.username, s.os, s.connected
        );
    }

    // Disconnect agent 1 after 3 seconds.
    println!("\nDesconectando PC-LAB-01 en 3 segundos...");
    tokio::time::sleep(Duration::from_secs(3)).await;
    cancel_a1.cancel();

    // Wait for the heartbeat timeout to detect the disconnection.
    println!("Esperando detección de desconexión (timeout de heartbeat)...");
    loop {
        if let Some(SessionEvent::Disconnected(id)) = events.recv().await {
            println!("Agente desconectado: sesión {id}");
            break;
        }
    }

    println!("\nRegistro final:");
    for s in registry.snapshot().await {
        println!(
            "  - {} | {} | conectado: {}",
            s.hostname, s.username, s.connected
        );
    }

    println!("\nConectados: {}", registry.connected_count().await);

    cancel.cancel();
    Ok(())
}
