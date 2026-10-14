//! Ed25519 key management and authentication helpers.
//!
//! The console holds an Ed25519 signing keypair. The agent holds the
//! corresponding verifying (public) key, configured at install time.

use std::path::Path;

use base64::Engine;
use ed25519_dalek::{Signature, Signer, SigningKey, Verifier, VerifyingKey};

use crate::error::CoreError;

const B64: base64::engine::GeneralPurpose = base64::engine::general_purpose::STANDARD;

// ---------------------------------------------------------------------------
// Key generation
// ---------------------------------------------------------------------------

/// Generate a new Ed25519 signing keypair.
pub fn generate_keypair() -> SigningKey {
    let mut rng = rand::thread_rng();
    SigningKey::generate(&mut rng)
}

/// Return the public key as a base64-encoded string (for display / config).
pub fn public_key_base64(key: &VerifyingKey) -> String {
    B64.encode(key.as_bytes())
}

/// Parse a public key from a base64-encoded string.
pub fn parse_public_key(b64: &str) -> Result<VerifyingKey, CoreError> {
    let bytes = B64
        .decode(b64.trim())
        .map_err(|e| CoreError::Auth(format!("invalid base64 for public key: {e}")))?;
    let bytes: [u8; 32] = bytes
        .try_into()
        .map_err(|_| CoreError::Auth("public key must be exactly 32 bytes".into()))?;
    VerifyingKey::from_bytes(&bytes)
        .map_err(|e| CoreError::Auth(format!("invalid Ed25519 public key: {e}")))
}

// ---------------------------------------------------------------------------
// Persistence
// ---------------------------------------------------------------------------

/// Save an Ed25519 keypair to disk.
///
/// The private key file contains the 32-byte seed encoded in base64.
/// The public key file contains the 32-byte verifying key in base64.
///
/// **The private key file is created with restricted permissions on Unix.**
pub fn save_keypair(
    signing_key: &SigningKey,
    private_path: &Path,
    public_path: &Path,
) -> Result<(), CoreError> {
    use std::io::Write;

    // Public key (safe to share).
    let pub_b64 = public_key_base64(&signing_key.verifying_key());
    std::fs::write(public_path, pub_b64.as_bytes())?;

    // Private key (secret).
    let priv_b64 = B64.encode(signing_key.to_bytes());
    {
        let mut f = std::fs::File::create(private_path)?;
        #[cfg(unix)]
        {
            use std::os::unix::fs::PermissionsExt;
            f.set_permissions(std::fs::Permissions::from_mode(0o600))?;
        }
        f.write_all(priv_b64.as_bytes())?;
    }

    Ok(())
}

/// Load an Ed25519 signing key from a file containing the 32-byte seed in
/// base64.
pub fn load_signing_key(private_path: &Path) -> Result<SigningKey, CoreError> {
    let contents = std::fs::read_to_string(private_path)?;
    let bytes = B64
        .decode(contents.trim())
        .map_err(|e| CoreError::Auth(format!("invalid base64 in private key file: {e}")))?;
    let bytes: [u8; 32] = bytes
        .try_into()
        .map_err(|_| CoreError::Auth("private key seed must be exactly 32 bytes".into()))?;
    Ok(SigningKey::from_bytes(&bytes))
}

/// Load an Ed25519 verifying (public) key from a file containing the 32-byte
/// key in base64.
pub fn load_verifying_key(public_path: &Path) -> Result<VerifyingKey, CoreError> {
    let contents = std::fs::read_to_string(public_path)?;
    parse_public_key(&contents)
}

// ---------------------------------------------------------------------------
// Nonce generation
// ---------------------------------------------------------------------------

/// Generate a random 32-byte nonce and return it as raw bytes.
pub fn generate_nonce() -> [u8; 32] {
    use rand::RngCore;
    let mut nonce = [0u8; 32];
    rand::thread_rng().fill_bytes(&mut nonce);
    nonce
}

/// Encode raw nonce bytes to base64 for transmission.
pub fn encode_nonce(nonce: &[u8]) -> String {
    B64.encode(nonce)
}

/// Decode a base64-encoded nonce back to raw bytes.
pub fn decode_nonce(b64: &str) -> Result<Vec<u8>, CoreError> {
    B64.decode(b64)
        .map_err(|e| CoreError::Auth(format!("invalid base64 nonce: {e}")))
}

// ---------------------------------------------------------------------------
// Signing and verification
// ---------------------------------------------------------------------------

/// Sign raw nonce bytes with the console's Ed25519 private key.
/// Returns the signature as a base64-encoded string.
pub fn sign_nonce(signing_key: &SigningKey, nonce: &[u8]) -> String {
    let signature = signing_key.sign(nonce);
    B64.encode(signature.to_bytes())
}

/// Verify an Ed25519 signature over the given nonce bytes.
///
/// `signature_b64` is the base64-encoded 64-byte Ed25519 signature.
pub fn verify_signature(
    verifying_key: &VerifyingKey,
    nonce: &[u8],
    signature_b64: &str,
) -> Result<(), CoreError> {
    let sig_bytes = B64
        .decode(signature_b64)
        .map_err(|e| CoreError::Auth(format!("invalid base64 signature: {e}")))?;
    let sig_bytes: [u8; 64] = sig_bytes
        .try_into()
        .map_err(|_| CoreError::Auth("signature must be exactly 64 bytes".into()))?;
    let signature = Signature::from_bytes(&sig_bytes);

    verifying_key
        .verify(nonce, &signature)
        .map_err(|_| CoreError::AuthFailed)
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn keypair_roundtrip() {
        let key = generate_keypair();
        let dir = std::env::temp_dir().join("aula-auth-test");
        let _ = std::fs::create_dir_all(&dir);
        let priv_path = dir.join("test.key");
        let pub_path = dir.join("test.pub");

        save_keypair(&key, &priv_path, &pub_path).unwrap();

        let loaded_priv = load_signing_key(&priv_path).unwrap();
        let loaded_pub = load_verifying_key(&pub_path).unwrap();

        assert_eq!(key.to_bytes(), loaded_priv.to_bytes());
        assert_eq!(key.verifying_key(), loaded_pub);

        let _ = std::fs::remove_dir_all(&dir);
    }

    #[test]
    fn sign_and_verify_succeeds() {
        let key = generate_keypair();
        let nonce = generate_nonce();
        let sig_b64 = sign_nonce(&key, &nonce);
        let result = verify_signature(&key.verifying_key(), &nonce, &sig_b64);
        assert!(result.is_ok());
    }

    #[test]
    fn verify_with_wrong_key_fails() {
        let key1 = generate_keypair();
        let key2 = generate_keypair();
        let nonce = generate_nonce();
        let sig_b64 = sign_nonce(&key1, &nonce);
        // Verify with key2's public key — should fail.
        let result = verify_signature(&key2.verifying_key(), &nonce, &sig_b64);
        assert!(result.is_err());
    }

    #[test]
    fn verify_with_wrong_nonce_fails() {
        let key = generate_keypair();
        let nonce1 = generate_nonce();
        let nonce2 = generate_nonce();
        let sig_b64 = sign_nonce(&key, &nonce1);
        let result = verify_signature(&key.verifying_key(), &nonce2, &sig_b64);
        assert!(result.is_err());
    }

    #[test]
    fn public_key_base64_roundtrip() {
        let key = generate_keypair();
        let b64 = public_key_base64(&key.verifying_key());
        let parsed = parse_public_key(&b64).unwrap();
        assert_eq!(key.verifying_key(), parsed);
    }

    #[test]
    fn nonce_encode_decode() {
        let nonce = generate_nonce();
        let b64 = encode_nonce(&nonce);
        let decoded = decode_nonce(&b64).unwrap();
        assert_eq!(&nonce[..], &decoded[..]);
    }
}
