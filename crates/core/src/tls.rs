//! TLS certificate generation, loading, fingerprinting, and configuration.
//!
//! The console generates a self-signed ECDSA P-256 certificate on first run
//! and persists it.  The agent verifies the certificate by comparing its
//! SHA-256 fingerprint against the value received in the UDP announcement
//! (certificate pinning).

use std::io::BufReader;
use std::path::Path;
use std::sync::Arc;

use rustls::pki_types::{CertificateDer, PrivateKeyDer, ServerName};
use sha2::{Digest, Sha256};

use crate::error::CoreError;

// ---------------------------------------------------------------------------
// TLS identity (certificate + key)
// ---------------------------------------------------------------------------

/// A console's TLS identity: certificate chain and private key.
pub struct TlsIdentity {
    pub certs: Vec<CertificateDer<'static>>,
    pub key: PrivateKeyDer<'static>,
    pub fingerprint: String,
}

// ---------------------------------------------------------------------------
// Certificate generation
// ---------------------------------------------------------------------------

/// Generate a self-signed ECDSA certificate for the console.
///
/// The certificate subject CN is set to `console_name`.
pub fn generate_self_signed(console_name: &str) -> Result<TlsIdentity, CoreError> {
    let key_pair = rcgen::KeyPair::generate_for(&rcgen::PKCS_ECDSA_P256_SHA256)
        .map_err(|e| CoreError::Tls(format!("key generation failed: {e}")))?;

    let mut params = rcgen::CertificateParams::new(vec![])
        .map_err(|e| CoreError::Tls(format!("cert params failed: {e}")))?;
    params
        .distinguished_name
        .push(rcgen::DnType::CommonName, console_name);
    // Valid for ~10 years.
    params.not_after = rcgen::date_time_ymd(2036, 1, 1);

    let cert = params
        .self_signed(&key_pair)
        .map_err(|e| CoreError::Tls(format!("self-signed cert failed: {e}")))?;

    let cert_der = CertificateDer::from(cert.der().to_vec());
    let key_der = PrivateKeyDer::Pkcs8(key_pair.serialize_der().into());

    let fingerprint = compute_fingerprint(&cert_der);

    Ok(TlsIdentity {
        certs: vec![cert_der],
        key: key_der,
        fingerprint,
    })
}

// ---------------------------------------------------------------------------
// Persistence (PEM files)
// ---------------------------------------------------------------------------

/// Save a TLS identity to PEM files on disk.
pub fn save_identity(
    identity: &TlsIdentity,
    cert_path: &Path,
    key_path: &Path,
) -> Result<(), CoreError> {
    use base64::Engine;
    use std::io::Write;

    let b64 = base64::engine::general_purpose::STANDARD;

    // Write certificate PEM.
    let cert_b64 = b64.encode(identity.certs[0].as_ref());
    let cert_pem = format!(
        "-----BEGIN CERTIFICATE-----\n{}\n-----END CERTIFICATE-----\n",
        wrap_base64(&cert_b64)
    );
    std::fs::write(cert_path, cert_pem.as_bytes())?;

    // Write private key PEM.
    let key_bytes = match &identity.key {
        PrivateKeyDer::Pkcs8(data) => data.secret_pkcs8_der().to_vec(),
        _ => return Err(CoreError::Tls("unsupported private key format".to_string())),
    };
    let key_b64 = b64.encode(&key_bytes);
    let key_pem = format!(
        "-----BEGIN PRIVATE KEY-----\n{}\n-----END PRIVATE KEY-----\n",
        wrap_base64(&key_b64)
    );
    {
        let mut f = std::fs::File::create(key_path)?;
        // On Unix, restrict permissions before writing.
        #[cfg(unix)]
        {
            use std::os::unix::fs::PermissionsExt;
            f.set_permissions(std::fs::Permissions::from_mode(0o600))?;
        }
        f.write_all(key_pem.as_bytes())?;
    }

    Ok(())
}

/// Wrap a base64 string at 76 characters per line (PEM standard).
fn wrap_base64(s: &str) -> String {
    s.as_bytes()
        .chunks(76)
        .map(|chunk| std::str::from_utf8(chunk).unwrap_or(""))
        .collect::<Vec<_>>()
        .join("\n")
}

/// Load a TLS identity from PEM files.
pub fn load_identity(cert_path: &Path, key_path: &Path) -> Result<TlsIdentity, CoreError> {
    // Read certificate.
    let cert_file = std::fs::File::open(cert_path)?;
    let mut cert_reader = BufReader::new(cert_file);
    let certs: Vec<CertificateDer<'static>> = rustls_pemfile::certs(&mut cert_reader)
        .collect::<Result<Vec<_>, _>>()
        .map_err(|e| CoreError::Tls(format!("failed to parse cert PEM: {e}")))?;

    if certs.is_empty() {
        return Err(CoreError::Tls("no certificates found in PEM file".into()));
    }

    // Read private key.
    let key_file = std::fs::File::open(key_path)?;
    let mut key_reader = BufReader::new(key_file);
    let key = rustls_pemfile::private_key(&mut key_reader)
        .map_err(|e| CoreError::Tls(format!("failed to parse key PEM: {e}")))?
        .ok_or_else(|| CoreError::Tls("no private key found in PEM file".into()))?;

    let fingerprint = compute_fingerprint(&certs[0]);

    Ok(TlsIdentity {
        certs,
        key,
        fingerprint,
    })
}

// ---------------------------------------------------------------------------
// Fingerprinting
// ---------------------------------------------------------------------------

/// Compute the `SHA256:<hex>` fingerprint of a DER-encoded certificate.
pub fn compute_fingerprint(cert_der: &CertificateDer<'_>) -> String {
    let hash = Sha256::digest(cert_der.as_ref());
    let hex: String = hash.iter().map(|b| format!("{b:02x}")).collect();
    format!("SHA256:{hex}")
}

// ---------------------------------------------------------------------------
// Server TLS config
// ---------------------------------------------------------------------------

/// Build a `rustls::ServerConfig` from a TLS identity.
pub fn server_config(identity: &TlsIdentity) -> Result<Arc<rustls::ServerConfig>, CoreError> {
    let config = rustls::ServerConfig::builder_with_provider(Arc::new(
        rustls::crypto::ring::default_provider(),
    ))
    .with_safe_default_protocol_versions()
    .map_err(|e| CoreError::Tls(format!("protocol versions: {e}")))?
    .with_no_client_auth()
    .with_single_cert(identity.certs.clone(), identity.key.clone_key())
    .map_err(|e| CoreError::Tls(format!("server cert config: {e}")))?;

    Ok(Arc::new(config))
}

// ---------------------------------------------------------------------------
// Client TLS config (certificate pinning)
// ---------------------------------------------------------------------------

/// Build a `rustls::ClientConfig` that accepts only a certificate matching
/// the given SHA-256 fingerprint.
pub fn client_config_pinned(
    expected_fingerprint: &str,
) -> Result<Arc<rustls::ClientConfig>, CoreError> {
    let verifier = FingerprintVerifier {
        expected: expected_fingerprint.to_string(),
    };

    let config = rustls::ClientConfig::builder_with_provider(Arc::new(
        rustls::crypto::ring::default_provider(),
    ))
    .with_safe_default_protocol_versions()
    .map_err(|e| CoreError::Tls(format!("protocol versions: {e}")))?
    .dangerous()
    .with_custom_certificate_verifier(Arc::new(verifier))
    .with_no_client_auth();

    Ok(Arc::new(config))
}

/// Custom certificate verifier that accepts a certificate only if its
/// SHA-256 fingerprint matches the expected value (certificate pinning).
#[derive(Debug)]
struct FingerprintVerifier {
    expected: String,
}

impl rustls::client::danger::ServerCertVerifier for FingerprintVerifier {
    fn verify_server_cert(
        &self,
        end_entity: &CertificateDer<'_>,
        _intermediates: &[CertificateDer<'_>],
        _server_name: &ServerName<'_>,
        _ocsp_response: &[u8],
        _now: rustls::pki_types::UnixTime,
    ) -> Result<rustls::client::danger::ServerCertVerified, rustls::Error> {
        let actual = compute_fingerprint(end_entity);
        if actual == self.expected {
            Ok(rustls::client::danger::ServerCertVerified::assertion())
        } else {
            tracing::warn!(
                expected = %self.expected,
                actual = %actual,
                "TLS certificate fingerprint mismatch — rejecting connection"
            );
            Err(rustls::Error::General(format!(
                "certificate fingerprint mismatch: expected {}, got {}",
                self.expected, actual
            )))
        }
    }

    fn verify_tls12_signature(
        &self,
        message: &[u8],
        cert: &CertificateDer<'_>,
        dss: &rustls::DigitallySignedStruct,
    ) -> Result<rustls::client::danger::HandshakeSignatureValid, rustls::Error> {
        rustls::crypto::verify_tls12_signature(
            message,
            cert,
            dss,
            &rustls::crypto::ring::default_provider().signature_verification_algorithms,
        )
    }

    fn verify_tls13_signature(
        &self,
        message: &[u8],
        cert: &CertificateDer<'_>,
        dss: &rustls::DigitallySignedStruct,
    ) -> Result<rustls::client::danger::HandshakeSignatureValid, rustls::Error> {
        rustls::crypto::verify_tls13_signature(
            message,
            cert,
            dss,
            &rustls::crypto::ring::default_provider().signature_verification_algorithms,
        )
    }

    fn supported_verify_schemes(&self) -> Vec<rustls::SignatureScheme> {
        rustls::crypto::ring::default_provider()
            .signature_verification_algorithms
            .supported_schemes()
    }
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn generate_and_fingerprint() {
        let id = generate_self_signed("Test Console").unwrap();
        assert!(id.fingerprint.starts_with("SHA256:"));
        assert_eq!(id.fingerprint.len(), 7 + 64); // "SHA256:" + 64 hex chars
        assert!(!id.certs.is_empty());
    }

    #[test]
    fn fingerprint_is_deterministic() {
        let id = generate_self_signed("Test").unwrap();
        let fp1 = compute_fingerprint(&id.certs[0]);
        let fp2 = compute_fingerprint(&id.certs[0]);
        assert_eq!(fp1, fp2);
    }

    #[test]
    fn server_config_builds() {
        let id = generate_self_signed("Test").unwrap();
        let cfg = server_config(&id);
        assert!(cfg.is_ok());
    }

    #[test]
    fn client_config_pinned_builds() {
        let cfg = client_config_pinned(
            "SHA256:0000000000000000000000000000000000000000000000000000000000000000",
        );
        assert!(cfg.is_ok());
    }

    #[test]
    fn save_and_load_identity() {
        let id = generate_self_signed("Save Test").unwrap();
        let dir = std::env::temp_dir().join("aula-tls-test");
        let _ = std::fs::create_dir_all(&dir);
        let cert_path = dir.join("cert.pem");
        let key_path = dir.join("key.pem");

        save_identity(&id, &cert_path, &key_path).unwrap();

        let loaded = load_identity(&cert_path, &key_path).unwrap();
        assert_eq!(id.fingerprint, loaded.fingerprint);
        assert_eq!(id.certs.len(), loaded.certs.len());

        // Cleanup.
        let _ = std::fs::remove_dir_all(&dir);
    }
}
