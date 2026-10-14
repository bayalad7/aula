//! Key generation utility for Aula console.
//!
//! Generates an Ed25519 keypair for console identity and (optionally) a
//! self-signed TLS certificate.
//!
//! Usage:
//!   cargo run --example keygen -- [output_dir]
//!
//! Without arguments, keys are written to the current directory.
//! Files created:
//!   - console_ed25519.key  (private key — keep secret!)
//!   - console_ed25519.pub  (public key — copy to agent config)
//!   - console_tls_cert.pem (TLS certificate)
//!   - console_tls_key.pem  (TLS private key)

use std::path::PathBuf;

use aula_core::auth;
use aula_core::tls;

fn main() -> anyhow::Result<()> {
    let dir: PathBuf = std::env::args()
        .nth(1)
        .map(PathBuf::from)
        .unwrap_or_else(|| PathBuf::from("."));

    std::fs::create_dir_all(&dir)?;

    // --- Ed25519 keypair ---
    let signing_key = auth::generate_keypair();
    let pub_b64 = auth::public_key_base64(&signing_key.verifying_key());

    let priv_path = dir.join("console_ed25519.key");
    let pub_path = dir.join("console_ed25519.pub");
    auth::save_keypair(&signing_key, &priv_path, &pub_path)?;

    println!("Ed25519 keypair generated:");
    println!("  Private key: {}", priv_path.display());
    println!("  Public key:  {}", pub_path.display());
    println!();
    println!("Public key (base64 — copy to agent config):");
    println!("  {pub_b64}");
    println!();

    // --- TLS certificate ---
    let tls_identity = tls::generate_self_signed("Aula Console")?;

    let cert_path = dir.join("console_tls_cert.pem");
    let key_path = dir.join("console_tls_key.pem");
    tls::save_identity(&tls_identity, &cert_path, &key_path)?;

    println!("TLS certificate generated:");
    println!("  Certificate: {}", cert_path.display());
    println!("  Private key: {}", key_path.display());
    println!("  Fingerprint: {}", tls_identity.fingerprint);

    Ok(())
}
